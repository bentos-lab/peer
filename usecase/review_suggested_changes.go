package usecase

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"bentos-backend/domain"
)

const (
	suggestedChangesOptOutMarker = "[no-suggest]"
)

var findingSeverityPriority = map[domain.FindingSeverityEnum]int{
	domain.FindingSeverityCritical: 4,
	domain.FindingSeverityMajor:    3,
	domain.FindingSeverityMinor:    2,
	domain.FindingSeverityNit:      1,
}

func normalizeSuggestedChangesConfig(cfg SuggestedChangesConfig) SuggestedChangesConfig {
	if cfg.MinSeverity == "" {
		cfg.MinSeverity = domain.FindingSeverityMajor
	}
	if cfg.MaxCandidates <= 0 {
		cfg.MaxCandidates = 50
	}
	if cfg.MaxGroupSize <= 0 {
		cfg.MaxGroupSize = 5
	}
	if cfg.MaxWorkers <= 0 {
		cfg.MaxWorkers = 3
	}
	if cfg.GroupTimeout <= 0 {
		cfg.GroupTimeout = 20 * time.Second
	}
	if cfg.GenerateTimeout <= 0 {
		cfg.GenerateTimeout = 30 * time.Second
	}
	return cfg
}

func (u *reviewUseCase) attachSuggestedChanges(ctx context.Context, input domain.ReviewInput, findings []domain.Finding) []domain.Finding {
	if u.suggestionGrouping == nil || u.suggestedChangeGenerator == nil {
		u.logger.Warnf("Suggested changes are enabled, but one or more suggest-phase dependencies are missing.")
		return findings
	}

	filterStartedAt := time.Now()
	candidates := filterSuggestionCandidates(input, findings, u.suggestedChangesConfig)
	u.logger.Debugf("Stage %q produced %d candidates in %d ms.", "filter_suggestion_candidates", len(candidates), time.Since(filterStartedAt).Milliseconds())
	if len(candidates) == 0 {
		return findings
	}

	groupStartedAt := time.Now()
	groupCtx, cancelGrouping := context.WithTimeout(ctx, u.suggestedChangesConfig.GroupTimeout)
	groupResult, groupingErr := u.suggestionGrouping.GroupFindings(groupCtx, LLMSuggestionGroupingPayload{
		Input:        input,
		Candidates:   candidates,
		MaxGroupSize: u.suggestedChangesConfig.MaxGroupSize,
	})
	cancelGrouping()
	if groupingErr != nil || !isValidGrouping(groupResult, candidates, u.suggestedChangesConfig.MaxGroupSize) {
		if groupingErr != nil {
			u.logger.Warnf("LLM grouping failed. Falling back to deterministic grouping.")
			u.logger.Debugf("Failure details: %v.", groupingErr)
		} else {
			u.logger.Warnf("LLM grouping output is invalid. Falling back to deterministic grouping.")
		}
		groupResult = LLMSuggestionGroupingResult{
			Groups: deterministicGroups(candidates, u.suggestedChangesConfig.MaxGroupSize),
		}
	}
	u.logger.Debugf("Stage %q produced %d groups in %d ms.", "group_suggestion_candidates", len(groupResult.Groups), time.Since(groupStartedAt).Milliseconds())

	generateStartedAt := time.Now()
	suggestions := u.generateSuggestionsByGroup(ctx, input, groupResult.Groups, candidates)
	u.logger.Debugf("Stage %q generated %d suggested changes in %d ms.", "generate_suggested_changes", len(suggestions), time.Since(generateStartedAt).Milliseconds())
	if len(suggestions) == 0 {
		return findings
	}

	merged := make([]domain.Finding, 0, len(findings))
	for _, finding := range findings {
		updated := finding
		key := findingDeterministicKey(finding)
		if suggestion, ok := suggestions[key]; ok {
			suggestionCopy := suggestion
			updated.SuggestedChange = &suggestionCopy
		}
		merged = append(merged, updated)
	}

	return merged
}

func filterSuggestionCandidates(input domain.ReviewInput, findings []domain.Finding, cfg SuggestedChangesConfig) []SuggestionFindingCandidate {
	diffByPath := map[string]string{}
	for _, file := range input.ChangedFiles {
		diffSnippet := strings.TrimSpace(file.DiffSnippet)
		if diffSnippet == "" {
			diffSnippet = strings.TrimSpace(file.Content)
		}
		diffByPath[file.Path] = diffSnippet
	}

	out := make([]SuggestionFindingCandidate, 0, len(findings))
	seen := map[string]struct{}{}
	for _, finding := range findings {
		if strings.TrimSpace(finding.FilePath) == "" {
			continue
		}
		if finding.StartLine <= 0 || finding.EndLine <= 0 || finding.StartLine > finding.EndLine {
			continue
		}
		if containsSuggestionOptOut(finding) {
			continue
		}
		if !isSeverityAtLeast(finding.Severity, cfg.MinSeverity) {
			continue
		}

		signature := findingLegacyKey(finding)
		if _, exists := seen[signature]; exists {
			continue
		}
		seen[signature] = struct{}{}

		out = append(out, SuggestionFindingCandidate{
			Key:         suggestionCandidateKey(len(out)),
			Finding:     finding,
			DiffSnippet: diffByPath[finding.FilePath],
		})

		if len(out) >= cfg.MaxCandidates {
			break
		}
	}

	return out
}

func containsSuggestionOptOut(finding domain.Finding) bool {
	candidateText := strings.ToLower(strings.Join([]string{
		finding.Title,
		finding.Detail,
		finding.Suggestion,
	}, "\n"))
	return strings.Contains(candidateText, suggestedChangesOptOutMarker)
}

func isSeverityAtLeast(severity domain.FindingSeverityEnum, threshold domain.FindingSeverityEnum) bool {
	return findingSeverityPriority[severity] >= findingSeverityPriority[threshold]
}

func findingDeterministicKey(finding domain.Finding) string {
	return findingLegacyKey(finding)
}

func findingLegacyKey(finding domain.Finding) string {
	return fmt.Sprintf("%s:%d:%d:%s",
		strings.TrimSpace(finding.FilePath),
		finding.StartLine,
		finding.EndLine,
		strings.TrimSpace(finding.Title),
	)
}

func suggestionCandidateKey(index int) string {
	return fmt.Sprintf("finding-%d", index+1)
}

func findingAnchorKey(filePath string, startLine int, endLine int) string {
	return fmt.Sprintf("%s:%d:%d", strings.TrimSpace(filePath), startLine, endLine)
}

type parsedFindingKey struct {
	FilePath string
	Start    int
	End      int
	Title    string
}

func parseLegacyFindingKey(key string) (parsedFindingKey, bool) {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return parsedFindingKey{}, false
	}

	parts := strings.Split(trimmed, ":")
	if len(parts) < 3 {
		return parsedFindingKey{}, false
	}

	if len(parts) >= 4 {
		start, startErr := strconv.Atoi(parts[len(parts)-3])
		end, endErr := strconv.Atoi(parts[len(parts)-2])
		if startErr == nil && endErr == nil {
			filePath := strings.TrimSpace(strings.Join(parts[:len(parts)-3], ":"))
			if filePath == "" {
				return parsedFindingKey{}, false
			}
			return parsedFindingKey{
				FilePath: filePath,
				Start:    start,
				End:      end,
				Title:    strings.TrimSpace(parts[len(parts)-1]),
			}, true
		}
	}

	start, startErr := strconv.Atoi(parts[len(parts)-2])
	end, endErr := strconv.Atoi(parts[len(parts)-1])
	if startErr != nil || endErr != nil {
		return parsedFindingKey{}, false
	}
	filePath := strings.TrimSpace(strings.Join(parts[:len(parts)-2], ":"))
	if filePath == "" {
		return parsedFindingKey{}, false
	}
	return parsedFindingKey{
		FilePath: filePath,
		Start:    start,
		End:      end,
	}, true
}

func buildAnchorTitleKey(anchorKey string, title string) string {
	return fmt.Sprintf("%s|%s", anchorKey, strings.TrimSpace(title))
}

func resolveSuggestionCandidate(
	rawKey string,
	candidateByKey map[string]SuggestionFindingCandidate,
	candidateKeysByAnchor map[string][]string,
	candidateKeysByAnchorTitle map[string][]string,
) (SuggestionFindingCandidate, string) {
	if candidate, ok := candidateByKey[rawKey]; ok {
		return candidate, ""
	}

	parsedKey, parsed := parseLegacyFindingKey(rawKey)
	if !parsed {
		return SuggestionFindingCandidate{}, "unknown_key"
	}

	anchorKey := findingAnchorKey(parsedKey.FilePath, parsedKey.Start, parsedKey.End)
	if parsedKey.Title != "" {
		matches := candidateKeysByAnchorTitle[buildAnchorTitleKey(anchorKey, parsedKey.Title)]
		if len(matches) == 1 {
			return candidateByKey[matches[0]], ""
		}
		if len(matches) > 1 {
			return SuggestionFindingCandidate{}, "ambiguous_key"
		}
	}

	matches := candidateKeysByAnchor[anchorKey]
	if len(matches) == 1 {
		return candidateByKey[matches[0]], ""
	}
	if len(matches) > 1 {
		return SuggestionFindingCandidate{}, "ambiguous_key"
	}

	return SuggestionFindingCandidate{}, "unknown_key"
}

func deterministicGroups(candidates []SuggestionFindingCandidate, maxGroupSize int) []SuggestionFindingGroup {
	if len(candidates) == 0 {
		return nil
	}
	if maxGroupSize <= 0 {
		maxGroupSize = 5
	}

	sorted := append([]SuggestionFindingCandidate(nil), candidates...)
	sort.Slice(sorted, func(i, j int) bool {
		left := sorted[i].Finding
		right := sorted[j].Finding
		if left.FilePath != right.FilePath {
			return left.FilePath < right.FilePath
		}
		if left.StartLine != right.StartLine {
			return left.StartLine < right.StartLine
		}
		return left.Title < right.Title
	})

	const nearbyLineThreshold = 20
	groups := make([]SuggestionFindingGroup, 0)
	current := SuggestionFindingGroup{
		GroupID: fmt.Sprintf("group-%d", len(groups)+1),
	}
	currentFilePath := sorted[0].Finding.FilePath
	lastLine := 0

	flush := func() {
		if len(current.FindingKeys) == 0 {
			return
		}
		current.Rationale = "fallback deterministic grouping"
		groups = append(groups, current)
		current = SuggestionFindingGroup{
			GroupID: fmt.Sprintf("group-%d", len(groups)+1),
		}
	}

	for _, candidate := range sorted {
		needsNewGroup := false
		if len(current.FindingKeys) > 0 {
			if candidate.Finding.FilePath != currentFilePath {
				needsNewGroup = true
			}
			if candidate.Finding.StartLine-lastLine > nearbyLineThreshold {
				needsNewGroup = true
			}
			if len(current.FindingKeys) >= maxGroupSize {
				needsNewGroup = true
			}
		}
		if needsNewGroup {
			flush()
		}
		if len(current.FindingKeys) == 0 {
			currentFilePath = candidate.Finding.FilePath
		}
		current.FindingKeys = append(current.FindingKeys, candidate.Key)
		lastLine = candidate.Finding.StartLine
	}
	flush()

	return groups
}

func isValidGrouping(result LLMSuggestionGroupingResult, candidates []SuggestionFindingCandidate, maxGroupSize int) bool {
	if len(candidates) == 0 {
		return len(result.Groups) == 0
	}
	if len(result.Groups) == 0 {
		return false
	}
	if maxGroupSize <= 0 {
		maxGroupSize = 5
	}

	allowed := map[string]struct{}{}
	for _, candidate := range candidates {
		allowed[candidate.Key] = struct{}{}
	}
	seen := map[string]struct{}{}
	for _, group := range result.Groups {
		if len(group.FindingKeys) == 0 || len(group.FindingKeys) > maxGroupSize {
			return false
		}
		for _, key := range group.FindingKeys {
			if _, ok := allowed[key]; !ok {
				return false
			}
			if _, duplicate := seen[key]; duplicate {
				return false
			}
			seen[key] = struct{}{}
		}
	}
	return len(seen) == len(candidates)
}

func (u *reviewUseCase) generateSuggestionsByGroup(
	ctx context.Context,
	input domain.ReviewInput,
	groups []SuggestionFindingGroup,
	candidates []SuggestionFindingCandidate,
) map[string]domain.SuggestedChange {
	if len(groups) == 0 {
		return nil
	}

	candidateByKey := map[string]SuggestionFindingCandidate{}
	candidateKeysByAnchor := map[string][]string{}
	candidateKeysByAnchorTitle := map[string][]string{}
	for _, candidate := range candidates {
		candidateByKey[candidate.Key] = candidate
		anchorKey := findingAnchorKey(candidate.Finding.FilePath, candidate.Finding.StartLine, candidate.Finding.EndLine)
		candidateKeysByAnchor[anchorKey] = append(candidateKeysByAnchor[anchorKey], candidate.Key)
		anchorTitleKey := buildAnchorTitleKey(anchorKey, candidate.Finding.Title)
		candidateKeysByAnchorTitle[anchorTitleKey] = append(candidateKeysByAnchorTitle[anchorTitleKey], candidate.Key)
	}

	maxWorkers := u.suggestedChangesConfig.MaxWorkers
	if maxWorkers <= 0 {
		maxWorkers = 1
	}
	if maxWorkers > len(groups) {
		maxWorkers = len(groups)
	}

	groupChan := make(chan SuggestionFindingGroup)
	var wg sync.WaitGroup
	var mu sync.Mutex
	suggestionsByKey := map[string]domain.SuggestedChange{}
	generatedCount := 0
	matchedCount := 0
	droppedUnknownKey := 0
	droppedAmbiguousKey := 0
	droppedInvalidKind := 0
	droppedEmptyReplacement := 0
	droppedInvalidReplacement := 0

	worker := func() {
		defer wg.Done()
		for group := range groupChan {
			groupCandidates := make([]SuggestionFindingCandidate, 0, len(group.FindingKeys))
			for _, key := range group.FindingKeys {
				candidate, ok := candidateByKey[key]
				if ok {
					groupCandidates = append(groupCandidates, candidate)
				}
			}
			if len(groupCandidates) == 0 {
				continue
			}

			generateCtx, cancelGenerate := context.WithTimeout(ctx, u.suggestedChangesConfig.GenerateTimeout)
			groupDiffs := buildGroupDiffs(input, groupCandidates)
			generated, err := u.suggestedChangeGenerator.GenerateSuggestedChanges(generateCtx, LLMSuggestedChangePayload{
				Input:      input,
				Group:      group,
				Candidates: groupCandidates,
				GroupDiffs: groupDiffs,
			})
			cancelGenerate()
			if err != nil {
				u.logger.Warnf("Suggested change generation failed for group %q.", group.GroupID)
				u.logger.Debugf("Failure details: %v.", err)
				continue
			}

			validKeys := map[string]struct{}{}
			for _, candidate := range groupCandidates {
				validKeys[candidate.Key] = struct{}{}
			}
			for _, item := range generated.Suggestions {
				mu.Lock()
				generatedCount++
				mu.Unlock()

				candidate, resolveReason := resolveSuggestionCandidate(item.FindingKey, candidateByKey, candidateKeysByAnchor, candidateKeysByAnchorTitle)
				if resolveReason != "" {
					mu.Lock()
					if resolveReason == "ambiguous_key" {
						droppedAmbiguousKey++
					} else {
						droppedUnknownKey++
					}
					mu.Unlock()
					continue
				}
				if _, ok := validKeys[candidate.Key]; !ok {
					mu.Lock()
					droppedUnknownKey++
					mu.Unlock()
					continue
				}

				if item.SuggestedChange.Kind != domain.SuggestedChangeKindReplace && item.SuggestedChange.Kind != domain.SuggestedChangeKindDelete {
					mu.Lock()
					droppedInvalidKind++
					mu.Unlock()
					continue
				}
				if item.SuggestedChange.Kind == domain.SuggestedChangeKindReplace && strings.TrimSpace(item.SuggestedChange.Replacement) == "" {
					mu.Lock()
					droppedEmptyReplacement++
					mu.Unlock()
					continue
				}
				if item.SuggestedChange.Kind == domain.SuggestedChangeKindDelete && item.SuggestedChange.Replacement != "" {
					mu.Lock()
					droppedInvalidReplacement++
					mu.Unlock()
					continue
				}

				resolvedKey := findingLegacyKey(candidate.Finding)
				mu.Lock()
				suggestionsByKey[resolvedKey] = item.SuggestedChange
				matchedCount++
				mu.Unlock()
			}
		}
	}

	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go worker()
	}
	for _, group := range groups {
		groupChan <- group
	}
	close(groupChan)
	wg.Wait()

	u.logger.Debugf("Suggested-change validation stats: generated=%d matched=%d dropped_unknown_key=%d dropped_ambiguous_key=%d dropped_invalid_kind=%d dropped_empty_replacement=%d dropped_invalid_replacement=%d.",
		generatedCount, matchedCount, droppedUnknownKey, droppedAmbiguousKey, droppedInvalidKind, droppedEmptyReplacement, droppedInvalidReplacement)

	return suggestionsByKey
}

func buildGroupDiffs(input domain.ReviewInput, candidates []SuggestionFindingCandidate) []GroupFileDiffContext {
	diffByPath := map[string]string{}
	for _, changedFile := range input.ChangedFiles {
		diffSnippet := strings.TrimSpace(changedFile.DiffSnippet)
		if diffSnippet == "" {
			diffSnippet = strings.TrimSpace(changedFile.Content)
		}
		if diffSnippet == "" {
			continue
		}
		diffByPath[changedFile.Path] = diffSnippet
	}

	filePaths := map[string]struct{}{}
	for _, candidate := range candidates {
		filePath := strings.TrimSpace(candidate.Finding.FilePath)
		if filePath == "" {
			continue
		}
		filePaths[filePath] = struct{}{}
	}

	sortedPaths := make([]string, 0, len(filePaths))
	for filePath := range filePaths {
		sortedPaths = append(sortedPaths, filePath)
	}
	sort.Strings(sortedPaths)

	groupDiffs := make([]GroupFileDiffContext, 0, len(sortedPaths))
	for _, filePath := range sortedPaths {
		diffSnippet := diffByPath[filePath]
		if diffSnippet == "" {
			continue
		}
		groupDiffs = append(groupDiffs, GroupFileDiffContext{
			FilePath:    filePath,
			DiffSnippet: diffSnippet,
		})
	}

	return groupDiffs
}
