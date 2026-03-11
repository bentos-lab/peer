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
	"bentos-backend/shared/logger/stdlogger"
	sharedtext "bentos-backend/shared/text"
	"bentos-backend/usecase"
	"bentos-backend/usecase/contracts"
)

//go:embed overview_system.md
var overviewSystemPromptTemplateRaw string

//go:embed overview_input.md
var overviewUserPromptTemplateRaw string

// OverviewGenerator implements usecase.LLMOverviewGenerator via a generic LLM generator.
type OverviewGenerator struct {
	generator contracts.LLMGenerator
	logger    usecase.Logger
}

type overviewModelOutput struct {
	Categories   []domain.OverviewCategoryItem `json:"categories"`
	Walkthroughs []domain.OverviewWalkthrough  `json:"walkthroughs"`
}

type overviewUserPromptTemplateData struct {
	Title       string
	Description string
	Files       []overviewUserPromptFileData
}

type overviewUserPromptFileData struct {
	Path        string
	ChangedText string
}

// NewOverviewGenerator creates an overview generator backed by a generic LLM client.
func NewOverviewGenerator(generator contracts.LLMGenerator, logger usecase.Logger) (*OverviewGenerator, error) {
	if generator == nil {
		return nil, fmt.Errorf("llm generator must not be nil")
	}
	if logger == nil {
		logger = stdlogger.Nop()
	}
	return &OverviewGenerator{generator: generator, logger: logger}, nil
}

// GenerateOverview creates categorized and walkthrough overview data from changed content.
func (g *OverviewGenerator) GenerateOverview(ctx context.Context, payload usecase.LLMOverviewPayload) (usecase.LLMOverviewResult, error) {
	startedAt := time.Now()
	g.logger.Infof("LLM overview generation started.")
	if payload.Environment == nil {
		return usecase.LLMOverviewResult{}, fmt.Errorf("code environment must not be nil")
	}

	changedFiles, err := payload.Environment.LoadChangedFiles(ctx, domain.CodeEnvironmentLoadOptions{
		Base: payload.Input.Base,
		Head: payload.Input.Head,
	})
	if err != nil {
		return usecase.LLMOverviewResult{}, err
	}
	g.logger.Debugf("The overview input includes %d changed files.", len(changedFiles))

	systemPrompt, err := renderOverviewSystemPrompt()
	if err != nil {
		g.logger.Errorf("LLM overview generation failed while rendering the system prompt.")
		g.logger.Debugf("The LLM overview generation ran for %d ms before failing.", time.Since(startedAt).Milliseconds())
		g.logger.Debugf("Failure details: %v.", err)
		return usecase.LLMOverviewResult{}, err
	}

	userPrompt, err := renderOverviewUserPrompt(payload.Input, changedFiles)
	if err != nil {
		g.logger.Errorf("LLM overview generation failed while rendering the user prompt.")
		g.logger.Debugf("The LLM overview generation ran for %d ms before failing.", time.Since(startedAt).Milliseconds())
		g.logger.Debugf("Failure details: %v.", err)
		return usecase.LLMOverviewResult{}, err
	}

	outputMap, err := g.generator.GenerateJSON(ctx, contracts.GenerateParams{
		SystemPrompt: systemPrompt,
		Messages:     []string{userPrompt},
	}, overviewResponseSchema())
	if err != nil {
		g.logger.Errorf("LLM overview generation failed while requesting JSON output.")
		g.logger.Debugf("The LLM overview generation ran for %d ms before failing.", time.Since(startedAt).Milliseconds())
		g.logger.Debugf("Failure details: %v.", err)
		return usecase.LLMOverviewResult{}, err
	}

	raw, err := json.Marshal(outputMap)
	if err != nil {
		g.logger.Errorf("LLM overview generation failed while encoding model output.")
		g.logger.Debugf("The LLM overview generation ran for %d ms before failing.", time.Since(startedAt).Milliseconds())
		g.logger.Debugf("Failure details: %v.", err)
		return usecase.LLMOverviewResult{}, err
	}

	var decoded overviewModelOutput
	if err := json.Unmarshal(raw, &decoded); err != nil {
		err = fmt.Errorf("invalid overview model output: %w", err)
		g.logger.Errorf("LLM overview generation failed because the model output is invalid.")
		g.logger.Debugf("The LLM overview generation ran for %d ms before failing.", time.Since(startedAt).Milliseconds())
		g.logger.Debugf("Failure details: %v.", err)
		return usecase.LLMOverviewResult{}, err
	}

	if err := validateOverviewCategories(decoded.Categories); err != nil {
		g.logger.Errorf("LLM overview generation failed because one category is invalid.")
		g.logger.Debugf("The LLM overview generation ran for %d ms before failing.", time.Since(startedAt).Milliseconds())
		g.logger.Debugf("Failure details: %v.", err)
		return usecase.LLMOverviewResult{}, err
	}

	g.logger.Infof("LLM overview generation completed.")
	g.logger.Debugf("The LLM overview generation completed in %d ms.", time.Since(startedAt).Milliseconds())
	g.logger.Debugf("The LLM overview generation produced %d categories and %d walkthrough groups.", len(decoded.Categories), len(decoded.Walkthroughs))

	return usecase.LLMOverviewResult{
		Categories:   decoded.Categories,
		Walkthroughs: decoded.Walkthroughs,
	}, nil
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
