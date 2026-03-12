package codingagent

import (
	"context"
	"fmt"
	"strings"

	"bentos-backend/domain"
	"bentos-backend/usecase/contracts"
)

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

func normalizePromptRefs(base string, head string) (string, string) {
	normalizedBase := strings.TrimSpace(base)
	normalizedHead := strings.TrimSpace(head)
	if normalizedHead == "@staged" || normalizedHead == "@all" {
		return "", normalizedHead
	}
	return normalizedBase, normalizedHead
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

func overviewResponseSchema() map[string]any {
	categoryEnumValues := []string{
		string(domain.OverviewCategoryLogicUpdates),
		string(domain.OverviewCategoryRefactoring),
		string(domain.OverviewCategorySecurityFixes),
		string(domain.OverviewCategoryTestChanges),
		string(domain.OverviewCategoryDocumentation),
		string(domain.OverviewCategoryInfrastructureConfig),
	}

	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"categories", "walkthroughs"},
		"properties": map[string]any{
			"categories": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required":             []string{"category", "summary"},
					"properties": map[string]any{
						"category": map[string]any{"type": "string", "enum": categoryEnumValues},
						"summary":  map[string]any{"type": "string"},
					},
				},
			},
			"walkthroughs": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required":             []string{"groupName", "files", "summary"},
					"properties": map[string]any{
						"groupName": map[string]any{"type": "string"},
						"files":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
						"summary":   map[string]any{"type": "string"},
					},
				},
			},
		},
	}
}

func validateOverviewCategories(categories []domain.OverviewCategoryItem) error {
	allowed := map[domain.OverviewCategoryEnum]struct{}{
		domain.OverviewCategoryLogicUpdates:         {},
		domain.OverviewCategoryRefactoring:          {},
		domain.OverviewCategorySecurityFixes:        {},
		domain.OverviewCategoryTestChanges:          {},
		domain.OverviewCategoryDocumentation:        {},
		domain.OverviewCategoryInfrastructureConfig: {},
	}
	for _, category := range categories {
		if _, ok := allowed[category.Category]; !ok {
			return fmt.Errorf("invalid overview category: %q", category.Category)
		}
		if strings.TrimSpace(category.Summary) == "" {
			return fmt.Errorf("invalid overview category summary for %q", category.Category)
		}
	}
	return nil
}
