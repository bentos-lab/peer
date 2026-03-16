package codingagent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"bentos-backend/domain"
	"bentos-backend/usecase/contracts"
)

func normalizeSuggestedChange(change *domain.SuggestedChange) *domain.SuggestedChange {
	if change == nil {
		return nil
	}
	change.Kind = domain.SuggestedChangeKindEnum(strings.TrimSpace(string(change.Kind)))
	change.Reason = strings.TrimSpace(change.Reason)
	if change.StartLine <= 0 || change.EndLine <= 0 || change.StartLine > change.EndLine {
		return nil
	}
	if change.Kind != domain.SuggestedChangeKindReplace && change.Kind != domain.SuggestedChangeKindDelete {
		return nil
	}
	if change.Reason == "" {
		return nil
	}
	if change.Kind == domain.SuggestedChangeKindDelete {
		if strings.TrimSpace(change.Replacement) != "" {
			return nil
		}
		change.Replacement = ""
		return change
	}
	if strings.TrimSpace(change.Replacement) == "" {
		return nil
	}
	return change
}

func resolveLanguage(language string) string {
	trimmed := strings.TrimSpace(language)
	if trimmed == "" {
		return "English"
	}
	return trimmed
}

func ensureDiffContentAvailable(ctx context.Context, environment contracts.CodeEnvironment, base string, head string) error {
	changedFiles, err := environment.LoadChangedFiles(ctx, domain.CodeEnvironmentLoadOptions{
		Base: base,
		Head: head,
	})
	if err != nil {
		return fmt.Errorf("failed to load changed files: %w", err)
	}
	for _, file := range changedFiles {
		if strings.TrimSpace(file.DiffSnippet) != "" {
			return nil
		}
	}
	return fmt.Errorf("diff content is empty for base %q and head %q", base, head)
}

func (r *Reviewer) setupAgent(ctx context.Context, environment contracts.CodeEnvironment, head string) (contracts.CodingAgent, error) {
	agent, err := environment.SetupAgent(ctx, domain.CodingAgentSetupOptions{
		Agent: r.config.Agent,
		Ref:   strings.TrimSpace(head),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to setup coding agent: %w", err)
	}
	return agent, nil
}

func runTask(ctx context.Context, agent contracts.CodingAgent, cfg Config, task string) (string, error) {
	result, err := agent.Run(ctx, strings.TrimSpace(task), domain.CodingAgentRunOptions{
		Provider: cfg.Provider,
		Model:    cfg.Model,
	})
	if err != nil {
		return "", fmt.Errorf("failed to run coding agent task: %w", err)
	}
	return strings.TrimSpace(result.Text), nil
}

func decodeModelOutput(outputMap map[string]any, target any) error {
	raw, err := json.Marshal(outputMap)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("invalid formatted model output: %w", err)
	}
	return nil
}

func reviewResponseSchema() map[string]any {
	suggestedChangeSchema := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"startLine", "endLine", "kind", "replacement", "reason"},
		"properties": map[string]any{
			"startLine":   map[string]any{"type": "integer", "minimum": 1},
			"endLine":     map[string]any{"type": "integer", "minimum": 1},
			"kind":        map[string]any{"type": "string", "enum": []string{string(domain.SuggestedChangeKindReplace), string(domain.SuggestedChangeKindDelete)}},
			"replacement": map[string]any{"type": "string"},
			"reason":      map[string]any{"type": "string"},
		},
	}

	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"summary", "findings"},
		"properties": map[string]any{
			"summary": map[string]any{"type": "string"},
			"findings": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required":             []string{"filePath", "startLine", "endLine", "severity", "title", "detail", "suggestion"},
					"properties": map[string]any{
						"filePath":        map[string]any{"type": "string"},
						"startLine":       map[string]any{"type": "integer", "minimum": 1},
						"endLine":         map[string]any{"type": "integer", "minimum": 1},
						"severity":        map[string]any{"type": "string", "enum": []string{string(domain.FindingSeverityCritical), string(domain.FindingSeverityMajor), string(domain.FindingSeverityMinor), string(domain.FindingSeverityNit)}},
						"title":           map[string]any{"type": "string"},
						"detail":          map[string]any{"type": "string"},
						"suggestion":      map[string]any{"type": "string"},
						"suggestedChange": map[string]any{"anyOf": []any{suggestedChangeSchema, map[string]any{"type": "null"}}},
					},
				},
			},
		},
	}
}
