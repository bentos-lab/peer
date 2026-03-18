package llm

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/bentos-lab/peer/domain"
	sharedtext "github.com/bentos-lab/peer/shared/text"
)

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
						"category": map[string]any{
							"type": "string",
							"enum": categoryEnumValues,
						},
						"summary": map[string]any{
							"type": "string",
						},
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
						"groupName": map[string]any{
							"type": "string",
						},
						"files": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "string",
							},
						},
						"summary": map[string]any{
							"type": "string",
						},
					},
				},
			},
		},
	}
}

func renderOverviewSystemPrompt() (string, error) {
	parsedTemplate, err := template.New("overview_system_prompt").Parse(overviewSystemPromptTemplateRaw)
	if err != nil {
		return "", err
	}

	var rendered bytes.Buffer
	if err := parsedTemplate.Execute(&rendered, nil); err != nil {
		return "", err
	}

	return rendered.String(), nil
}

func renderOverviewUserPrompt(input domain.ChangeRequestInput, changedFiles []domain.ChangedFile) (string, error) {
	files := make([]overviewUserPromptFileData, 0, len(changedFiles))
	for _, file := range changedFiles {
		changedText := file.DiffSnippet
		if changedText == "" {
			changedText = file.Content
		}
		if changedText == "" {
			continue
		}
		files = append(files, overviewUserPromptFileData{
			Path:        file.Path,
			ChangedText: changedText,
		})
	}

	parsedTemplate, err := template.New("overview_user_prompt").Parse(overviewUserPromptTemplateRaw)
	if err != nil {
		return "", err
	}

	var rendered bytes.Buffer
	if err := parsedTemplate.Execute(&rendered, overviewUserPromptTemplateData{
		Title:       input.Title,
		Description: sharedtext.SingleLine(input.Description),
		Files:       files,
	}); err != nil {
		return "", err
	}

	return rendered.String(), nil
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
