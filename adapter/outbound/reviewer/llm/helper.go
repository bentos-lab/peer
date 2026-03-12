package llm

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"text/template"

	"bentos-backend/domain"
	sharedtext "bentos-backend/shared/text"
	"bentos-backend/usecase"
)

type lineRange struct {
	Start int
	End   int
}

var unifiedDiffHunkPattern = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@`)

func reviewResponseSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"summary", "findings"},
		"properties": map[string]any{
			"summary": map[string]any{
				"type": "string",
			},
			"findings": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required": []string{
						"filePath",
						"startLine",
						"endLine",
						"severity",
						"title",
						"detail",
						"suggestion",
					},
					"properties": map[string]any{
						"filePath": map[string]any{
							"type": "string",
						},
						"startLine": map[string]any{
							"type":    "integer",
							"minimum": 1,
						},
						"endLine": map[string]any{
							"type":    "integer",
							"minimum": 1,
						},
						"severity": map[string]any{
							"type": "string",
							"enum": []string{
								string(domain.FindingSeverityCritical),
								string(domain.FindingSeverityMajor),
								string(domain.FindingSeverityMinor),
								string(domain.FindingSeverityNit),
							},
						},
						"title": map[string]any{
							"type": "string",
						},
						"detail": map[string]any{
							"type": "string",
						},
						"suggestion": map[string]any{
							"type": "string",
						},
					},
				},
			},
		},
	}
}

func renderSystemPrompt(rulePackText string) (string, error) {
	parsedTemplate, err := template.New("reviewer_system_prompt").Parse(reviewSystemPromptTemplateRaw)
	if err != nil {
		return "", err
	}

	var rendered bytes.Buffer
	if err := parsedTemplate.Execute(&rendered, reviewSystemPromptTemplateData{
		RulePackText: rulePackText,
	}); err != nil {
		return "", err
	}

	return rendered.String(), nil
}

func renderUserPrompt(input domain.ChangeRequestInput, changedFiles []domain.ChangedFile) (string, error) {
	files := make([]reviewUserPromptFileData, 0, len(changedFiles))
	for _, file := range changedFiles {
		changedText := file.DiffSnippet
		if changedText == "" {
			changedText = file.Content
		}
		if changedText == "" {
			continue
		}
		files = append(files, reviewUserPromptFileData{
			Path:        file.Path,
			ChangedText: changedText,
		})
	}

	language := input.Language
	if language == "" {
		language = "English"
	}

	parsedTemplate, err := template.New("reviewer_user_prompt").Parse(reviewUserPromptTemplateRaw)
	if err != nil {
		return "", err
	}

	var rendered bytes.Buffer
	if err := parsedTemplate.Execute(&rendered, reviewUserPromptTemplateData{
		Title:       input.Title,
		Description: sharedtext.SingleLine(input.Description),
		Language:    language,
		Files:       files,
	}); err != nil {
		return "", err
	}

	return rendered.String(), nil
}

func buildChangedRangesByFile(changedFiles []domain.ChangedFile, logger usecase.Logger) map[string][]lineRange {
	rangesByFile := make(map[string][]lineRange, len(changedFiles))

	for _, file := range changedFiles {
		path := strings.TrimSpace(file.Path)
		if path == "" {
			continue
		}

		ranges, err := parseChangedRangesFromDiff(file.DiffSnippet)
		if err != nil {
			logger.Warnf("Skipping changed-line alignment metadata for file %q because diff parsing failed: %v.", path, err)
			continue
		}
		if len(ranges) == 0 {
			continue
		}
		rangesByFile[path] = ranges
	}

	return rangesByFile
}

func parseChangedRangesFromDiff(diffSnippet string) ([]lineRange, error) {
	diffSnippet = strings.TrimSpace(diffSnippet)
	if diffSnippet == "" {
		return nil, nil
	}

	lines := strings.Split(diffSnippet, "\n")
	changedLines := make([]int, 0)
	inHunk := false
	currentNewLine := 0
	seenHunk := false

	for _, rawLine := range lines {
		line := strings.TrimSuffix(rawLine, "\r")
		if matches := unifiedDiffHunkPattern.FindStringSubmatch(line); matches != nil {
			newStart, err := strconv.Atoi(matches[1])
			if err != nil {
				return nil, fmt.Errorf("invalid hunk new start %q", matches[1])
			}

			currentNewLine = newStart
			inHunk = true
			seenHunk = true
			continue
		}

		if !inHunk {
			continue
		}

		if strings.HasPrefix(line, "diff --git ") {
			inHunk = false
			continue
		}
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			changedLines = append(changedLines, currentNewLine)
			currentNewLine++
			continue
		}
		if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			continue
		}
		if strings.HasPrefix(line, " ") {
			currentNewLine++
			continue
		}
		if line == `\ No newline at end of file` {
			continue
		}
		if strings.HasPrefix(line, "--- ") || strings.HasPrefix(line, "+++ ") {
			inHunk = false
			continue
		}
	}

	if !seenHunk {
		return nil, fmt.Errorf("no unified diff hunk found")
	}

	return mergeLineNumbersToRanges(changedLines), nil
}

func mergeLineNumbersToRanges(lines []int) []lineRange {
	if len(lines) == 0 {
		return nil
	}

	ranges := make([]lineRange, 0, len(lines))
	current := lineRange{Start: lines[0], End: lines[0]}
	for _, line := range lines[1:] {
		if line <= 0 {
			continue
		}
		if line == current.End+1 {
			current.End = line
			continue
		}
		if line <= current.End {
			continue
		}
		ranges = append(ranges, current)
		current = lineRange{Start: line, End: line}
	}
	ranges = append(ranges, current)
	return ranges
}

func splitFindingsByChangedRanges(findings []domain.Finding, changedRangesByFile map[string][]lineRange, logger usecase.Logger) []domain.Finding {
	filtered := make([]domain.Finding, 0, len(findings))

	for _, finding := range findings {
		path := strings.TrimSpace(finding.FilePath)
		changedRanges := changedRangesByFile[path]
		segments := intersectRangeWithChangedLines(finding.StartLine, finding.EndLine, changedRanges)
		if len(segments) == 0 {
			logger.Warnf("Dropping finding because its range does not overlap changed lines file=%q startLine=%d endLine=%d title=%q.", finding.FilePath, finding.StartLine, finding.EndLine, finding.Title)
			continue
		}

		droppedSegments := 0
		if len(segments) > maxSplitFindingsPerOriginal {
			droppedSegments = len(segments) - maxSplitFindingsPerOriginal
			segments = segments[:maxSplitFindingsPerOriginal]
		}

		logger.Debugf("Aligned finding to changed lines file=%q startLine=%d endLine=%d splitCount=%d title=%q.",
			finding.FilePath, finding.StartLine, finding.EndLine, len(segments), finding.Title)
		if droppedSegments > 0 {
			logger.Warnf("Dropped split finding segments because split count exceeded limit file=%q startLine=%d endLine=%d kept=%d dropped=%d title=%q.",
				finding.FilePath, finding.StartLine, finding.EndLine, len(segments), droppedSegments, finding.Title)
		}

		for _, segment := range segments {
			derived := finding
			derived.StartLine = segment.Start
			derived.EndLine = segment.End
			filtered = append(filtered, derived)
		}
	}

	return filtered
}

func intersectRangeWithChangedLines(startLine int, endLine int, changedRanges []lineRange) []lineRange {
	if startLine <= 0 || endLine <= 0 || startLine > endLine || len(changedRanges) == 0 {
		return nil
	}

	result := make([]lineRange, 0)
	for _, changed := range changedRanges {
		if changed.End < startLine {
			continue
		}
		if changed.Start > endLine {
			break
		}
		overlapStart := startLine
		if changed.Start > overlapStart {
			overlapStart = changed.Start
		}
		overlapEnd := endLine
		if changed.End < overlapEnd {
			overlapEnd = changed.End
		}
		if overlapStart <= overlapEnd {
			result = append(result, lineRange{Start: overlapStart, End: overlapEnd})
		}
	}
	return result
}
