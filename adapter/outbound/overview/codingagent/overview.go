package codingagent

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
	sharedtext "bentos-backend/shared/text"
	"bentos-backend/shared/logger/stdlogger"
	"bentos-backend/usecase"
	"bentos-backend/usecase/contracts"
)

//go:embed task.md
var overviewTaskPromptTemplateRaw string

//go:embed formatting_system.md
var overviewFormattingSystemPrompt string

// Config contains coding-agent runtime options.
type Config struct {
	Agent    string
	Provider string
	Model    string
}

// OverviewGenerator uses coding agent analysis and LLM JSON formatting.
type OverviewGenerator struct {
	formatter contracts.LLMGenerator
	config    Config
	logger    usecase.Logger
}

type overviewTaskPromptTemplateData struct {
	Repository  string
	RepoURL     string
	Base        string
	Head        string
	Title       string
	Description string
}

type overviewModelOutput struct {
	Categories   []domain.OverviewCategoryItem `json:"categories"`
	Walkthroughs []domain.OverviewWalkthrough  `json:"walkthroughs"`
}

// NewOverviewGenerator creates a coding-agent overview adapter.
func NewOverviewGenerator(formatter contracts.LLMGenerator, config Config, logger usecase.Logger) (*OverviewGenerator, error) {
	if formatter == nil {
		return nil, fmt.Errorf("formatter llm generator must not be nil")
	}
	if strings.TrimSpace(config.Agent) == "" {
		return nil, fmt.Errorf("coding agent is required")
	}
	if strings.TrimSpace(config.Provider) == "" {
		return nil, fmt.Errorf("coding agent provider is required")
	}
	if strings.TrimSpace(config.Model) == "" {
		return nil, fmt.Errorf("coding agent model is required")
	}
	if logger == nil {
		logger = stdlogger.Nop()
	}
	return &OverviewGenerator{formatter: formatter, config: config, logger: logger}, nil
}

// GenerateOverview produces overview output from coding agent text.
func (g *OverviewGenerator) GenerateOverview(ctx context.Context, payload usecase.LLMOverviewPayload) (usecase.LLMOverviewResult, error) {
	startedAt := time.Now()
	g.logger.Infof("Coding-agent overview generation started.")

	normalizedBase, normalizedHead := normalizePromptRefs(payload.Input.Base, payload.Input.Head)

	if payload.Environment == nil {
		return usecase.LLMOverviewResult{}, fmt.Errorf("code environment must not be nil")
	}
	if err := ensureDiffContentAvailable(ctx, payload.Environment, normalizedBase, normalizedHead); err != nil {
		return usecase.LLMOverviewResult{}, err
	}

	agent, err := payload.Environment.SetupAgent(ctx, domain.CodingAgentSetupOptions{
		Agent: g.config.Agent,
		Ref:   normalizedHead,
	})
	if err != nil {
		return usecase.LLMOverviewResult{}, fmt.Errorf("failed to setup coding agent: %w", err)
	}
	taskPrompt, err := renderSimpleTemplate("overview_task_prompt", overviewTaskPromptTemplateRaw, overviewTaskPromptTemplateData{
		Repository:  payload.Input.Target.Repository,
		RepoURL:     payload.Input.RepoURL,
		Base:        normalizedBase,
		Head:        normalizedHead,
		Title:       payload.Input.Title,
		Description: sharedtext.SingleLine(payload.Input.Description),
	})
	if err != nil {
		return usecase.LLMOverviewResult{}, err
	}

	rawText, err := runTask(ctx, agent, g.config, taskPrompt)
	if err != nil {
		return usecase.LLMOverviewResult{}, err
	}

	outputMap, err := g.formatter.GenerateJSON(ctx, contracts.GenerateParams{
		SystemPrompt: overviewFormattingSystemPrompt,
		Messages: []contracts.Message{{
			Role:    "user",
			Content: rawText,
		}},
		ResponseSchema: overviewResponseSchema(),
	})
	if err != nil {
		return usecase.LLMOverviewResult{}, err
	}

	raw, err := json.Marshal(outputMap)
	if err != nil {
		return usecase.LLMOverviewResult{}, err
	}
	var decoded overviewModelOutput
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return usecase.LLMOverviewResult{}, fmt.Errorf("invalid formatted overview output: %w", err)
	}

	if err := validateOverviewCategories(decoded.Categories); err != nil {
		return usecase.LLMOverviewResult{}, err
	}

	g.logger.Infof("Coding-agent overview generation completed.")
	g.logger.Debugf("Coding-agent overview generation completed in %d ms.", time.Since(startedAt).Milliseconds())

	return usecase.LLMOverviewResult{Categories: decoded.Categories, Walkthroughs: decoded.Walkthroughs}, nil
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
