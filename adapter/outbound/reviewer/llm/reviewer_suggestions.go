package llm

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"
	"time"

	"bentos-backend/domain"
	"bentos-backend/usecase"
	"bentos-backend/usecase/contracts"
)

//go:embed grouping_system.md
var groupingSystemPromptTemplateRaw string

//go:embed grouping_input.md
var groupingUserPromptTemplateRaw string

//go:embed suggestion_system.md
var suggestionSystemPromptTemplateRaw string

//go:embed suggestion_input.md
var suggestionUserPromptTemplateRaw string

type groupingModelOutput struct {
	Groups []groupingOutputGroup `json:"groups"`
}

type groupingOutputGroup struct {
	GroupID     string   `json:"groupId"`
	Rationale   string   `json:"rationale"`
	FindingKeys []string `json:"findingKeys"`
}

type suggestionModelOutput struct {
	Suggestions []suggestionOutputItem `json:"suggestions"`
}

type suggestionOutputItem struct {
	FindingKey  string `json:"findingKey"`
	Kind        string `json:"kind"`
	Replacement string `json:"replacement"`
	Reason      string `json:"reason"`
}

type groupingUserPromptTemplateData struct {
	MaxGroupSize int
	Candidates   []groupingUserPromptCandidate
}

type groupingUserPromptCandidate struct {
	Key         string
	FilePath    string
	StartLine   int
	EndLine     int
	Severity    string
	Title       string
	Detail      string
	DiffSnippet string
}

type suggestionUserPromptTemplateData struct {
	GroupID    string
	Rationale  string
	GroupDiffs []suggestionUserPromptGroupDiff
	Candidates []suggestionUserPromptCandidate
}

type suggestionUserPromptGroupDiff struct {
	FilePath    string
	DiffSnippet string
}

type suggestionUserPromptCandidate struct {
	Key         string
	FilePath    string
	StartLine   int
	EndLine     int
	Severity    string
	Title       string
	Detail      string
	DiffSnippet string
}

// GroupFindings groups filtered findings into suggestion-ready batches.
func (r *Reviewer) GroupFindings(ctx context.Context, payload usecase.LLMSuggestionGroupingPayload) (usecase.LLMSuggestionGroupingResult, error) {
	startedAt := time.Now()
	r.logger.Infof("LLM grouping of suggestion candidates started.")
	r.logger.Debugf("Grouping payload includes %d candidates.", len(payload.Candidates))

	systemPrompt, err := renderSimpleTemplate("grouping_system_prompt", groupingSystemPromptTemplateRaw, map[string]any{})
	if err != nil {
		r.logger.Errorf("LLM grouping failed while rendering the system prompt.")
		r.logger.Debugf("The grouping stage ran for %d ms before failing.", time.Since(startedAt).Milliseconds())
		r.logger.Debugf("Failure details: %v.", err)
		return usecase.LLMSuggestionGroupingResult{}, err
	}

	userPrompt, err := renderGroupingUserPrompt(payload)
	if err != nil {
		r.logger.Errorf("LLM grouping failed while rendering the user prompt.")
		r.logger.Debugf("The grouping stage ran for %d ms before failing.", time.Since(startedAt).Milliseconds())
		r.logger.Debugf("Failure details: %v.", err)
		return usecase.LLMSuggestionGroupingResult{}, err
	}

	outputMap, err := r.generator.GenerateJSON(ctx, contracts.GenerateParams{
		SystemPrompt: systemPrompt,
		Messages: []contracts.Message{
			{Role: "user", Content: userPrompt},
		},
		ResponseSchema: groupingResponseSchema(),
	})
	if err != nil {
		r.logger.Errorf("LLM grouping failed while requesting JSON output.")
		r.logger.Debugf("The grouping stage ran for %d ms before failing.", time.Since(startedAt).Milliseconds())
		r.logger.Debugf("Failure details: %v.", err)
		return usecase.LLMSuggestionGroupingResult{}, err
	}

	raw, err := json.Marshal(outputMap)
	if err != nil {
		r.logger.Errorf("LLM grouping failed while encoding model output.")
		r.logger.Debugf("Failure details: %v.", err)
		return usecase.LLMSuggestionGroupingResult{}, err
	}

	var decoded groupingModelOutput
	if err := json.Unmarshal(raw, &decoded); err != nil {
		err = fmt.Errorf("invalid suggestion grouping model output: %w", err)
		r.logger.Errorf("LLM grouping failed because the model output is invalid.")
		r.logger.Debugf("Failure details: %v.", err)
		return usecase.LLMSuggestionGroupingResult{}, err
	}

	resultGroups := make([]usecase.SuggestionFindingGroup, 0, len(decoded.Groups))
	for _, group := range decoded.Groups {
		resultGroups = append(resultGroups, usecase.SuggestionFindingGroup{
			GroupID:     strings.TrimSpace(group.GroupID),
			Rationale:   strings.TrimSpace(group.Rationale),
			FindingKeys: append([]string(nil), group.FindingKeys...),
		})
	}

	r.logger.Infof("LLM grouping of suggestion candidates completed.")
	r.logger.Debugf("The grouping stage completed in %d ms and returned %d groups.", time.Since(startedAt).Milliseconds(), len(resultGroups))

	return usecase.LLMSuggestionGroupingResult{Groups: resultGroups}, nil
}

// GenerateSuggestedChanges creates suggested code changes for one grouped candidate set.
func (r *Reviewer) GenerateSuggestedChanges(ctx context.Context, payload usecase.LLMSuggestedChangePayload) (usecase.LLMSuggestedChangeResult, error) {
	startedAt := time.Now()
	r.logger.Infof("LLM suggested-change generation started.")
	r.logger.Debugf("Suggestion payload group %q includes %d candidates.", payload.Group.GroupID, len(payload.Candidates))

	systemPrompt, err := renderSimpleTemplate("suggestion_system_prompt", suggestionSystemPromptTemplateRaw, map[string]any{})
	if err != nil {
		r.logger.Errorf("Suggested-change generation failed while rendering the system prompt.")
		r.logger.Debugf("Failure details: %v.", err)
		return usecase.LLMSuggestedChangeResult{}, err
	}

	userPrompt, err := renderSuggestionUserPrompt(payload)
	if err != nil {
		r.logger.Errorf("Suggested-change generation failed while rendering the user prompt.")
		r.logger.Debugf("Failure details: %v.", err)
		return usecase.LLMSuggestedChangeResult{}, err
	}

	outputMap, err := r.generator.GenerateJSON(ctx, contracts.GenerateParams{
		SystemPrompt: systemPrompt,
		Messages: []contracts.Message{
			{Role: "user", Content: userPrompt},
		},
		ResponseSchema: suggestionResponseSchema(),
	})
	if err != nil {
		r.logger.Errorf("Suggested-change generation failed while requesting JSON output.")
		r.logger.Debugf("The generation stage ran for %d ms before failing.", time.Since(startedAt).Milliseconds())
		r.logger.Debugf("Failure details: %v.", err)
		return usecase.LLMSuggestedChangeResult{}, err
	}

	raw, err := json.Marshal(outputMap)
	if err != nil {
		r.logger.Errorf("Suggested-change generation failed while encoding model output.")
		r.logger.Debugf("Failure details: %v.", err)
		return usecase.LLMSuggestedChangeResult{}, err
	}

	var decoded suggestionModelOutput
	if err := json.Unmarshal(raw, &decoded); err != nil {
		err = fmt.Errorf("invalid suggested-change model output: %w", err)
		r.logger.Errorf("Suggested-change generation failed because the model output is invalid.")
		r.logger.Debugf("Failure details: %v.", err)
		return usecase.LLMSuggestedChangeResult{}, err
	}

	resultItems := make([]usecase.FindingSuggestedChange, 0, len(decoded.Suggestions))
	for _, item := range decoded.Suggestions {
		change := domain.SuggestedChange{
			Kind:        domain.SuggestedChangeKindEnum(strings.TrimSpace(item.Kind)),
			Replacement: item.Replacement,
			Reason:      strings.TrimSpace(item.Reason),
		}
		if change.Kind != domain.SuggestedChangeKindReplace && change.Kind != domain.SuggestedChangeKindDelete {
			continue
		}
		if change.Kind == domain.SuggestedChangeKindDelete && change.Replacement != "" {
			continue
		}
		if change.Kind == domain.SuggestedChangeKindReplace && strings.TrimSpace(change.Replacement) == "" {
			continue
		}
		if change.Reason == "" {
			continue
		}
		resultItems = append(resultItems, usecase.FindingSuggestedChange{
			FindingKey:      strings.TrimSpace(item.FindingKey),
			SuggestedChange: change,
		})
	}

	r.logger.Infof("LLM suggested-change generation completed.")
	r.logger.Debugf("The generation stage completed in %d ms and returned %d suggestions.", time.Since(startedAt).Milliseconds(), len(resultItems))

	return usecase.LLMSuggestedChangeResult{
		Suggestions: resultItems,
	}, nil
}

func groupingResponseSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"groups"},
		"properties": map[string]any{
			"groups": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required":             []string{"groupId", "rationale", "findingKeys"},
					"properties": map[string]any{
						"groupId": map[string]any{
							"type": "string",
						},
						"rationale": map[string]any{
							"type": "string",
						},
						"findingKeys": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "string",
							},
						},
					},
				},
			},
		},
	}
}

func suggestionResponseSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"suggestions"},
		"properties": map[string]any{
			"suggestions": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required":             []string{"findingKey", "kind", "replacement", "reason"},
					"properties": map[string]any{
						"findingKey": map[string]any{
							"type": "string",
						},
						"kind": map[string]any{
							"type": "string",
							"enum": []string{
								string(domain.SuggestedChangeKindReplace),
								string(domain.SuggestedChangeKindDelete),
							},
						},
						"replacement": map[string]any{
							"type": "string",
						},
						"reason": map[string]any{
							"type": "string",
						},
					},
				},
			},
		},
	}
}

func renderSimpleTemplate(templateName string, templateRaw string, input any) (string, error) {
	parsedTemplate, err := template.New(templateName).Parse(templateRaw)
	if err != nil {
		return "", err
	}
	var rendered bytes.Buffer
	if err := parsedTemplate.Execute(&rendered, input); err != nil {
		return "", err
	}
	return rendered.String(), nil
}

func renderGroupingUserPrompt(payload usecase.LLMSuggestionGroupingPayload) (string, error) {
	items := make([]groupingUserPromptCandidate, 0, len(payload.Candidates))
	for _, candidate := range payload.Candidates {
		items = append(items, groupingUserPromptCandidate{
			Key:         candidate.Key,
			FilePath:    candidate.Finding.FilePath,
			StartLine:   candidate.Finding.StartLine,
			EndLine:     candidate.Finding.EndLine,
			Severity:    string(candidate.Finding.Severity),
			Title:       candidate.Finding.Title,
			Detail:      candidate.Finding.Detail,
			DiffSnippet: candidate.DiffSnippet,
		})
	}

	return renderSimpleTemplate("grouping_user_prompt", groupingUserPromptTemplateRaw, groupingUserPromptTemplateData{
		MaxGroupSize: payload.MaxGroupSize,
		Candidates:   items,
	})
}

func renderSuggestionUserPrompt(payload usecase.LLMSuggestedChangePayload) (string, error) {
	groupDiffs := make([]suggestionUserPromptGroupDiff, 0, len(payload.GroupDiffs))
	for _, groupDiff := range payload.GroupDiffs {
		groupDiffs = append(groupDiffs, suggestionUserPromptGroupDiff{
			FilePath:    groupDiff.FilePath,
			DiffSnippet: groupDiff.DiffSnippet,
		})
	}

	items := make([]suggestionUserPromptCandidate, 0, len(payload.Candidates))
	for _, candidate := range payload.Candidates {
		items = append(items, suggestionUserPromptCandidate{
			Key:         candidate.Key,
			FilePath:    candidate.Finding.FilePath,
			StartLine:   candidate.Finding.StartLine,
			EndLine:     candidate.Finding.EndLine,
			Severity:    string(candidate.Finding.Severity),
			Title:       candidate.Finding.Title,
			Detail:      candidate.Finding.Detail,
			DiffSnippet: candidate.DiffSnippet,
		})
	}

	return renderSimpleTemplate("suggestion_user_prompt", suggestionUserPromptTemplateRaw, suggestionUserPromptTemplateData{
		GroupID:    payload.Group.GroupID,
		Rationale:  payload.Group.Rationale,
		GroupDiffs: groupDiffs,
		Candidates: items,
	})
}
