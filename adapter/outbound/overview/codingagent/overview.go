package codingagent

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"bentos-backend/domain"
	"bentos-backend/shared/logger/stdlogger"
	sharedtext "bentos-backend/shared/text"
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
	Repository    string
	RepoURL       string
	Base          string
	Head          string
	Title         string
	Description   string
	ExtraGuidance string
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
	taskPrompt, err := sharedtext.RenderSimpleTemplate("overview_task_prompt", overviewTaskPromptTemplateRaw, overviewTaskPromptTemplateData{
		Repository:    payload.Input.Target.Repository,
		RepoURL:       payload.Input.RepoURL,
		Base:          normalizedBase,
		Head:          normalizedHead,
		Title:         payload.Input.Title,
		Description:   sharedtext.SingleLine(payload.Input.Description),
		ExtraGuidance: strings.TrimSpace(payload.ExtraGuidance),
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
		Messages:     []string{rawText},
	}, overviewResponseSchema())
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

	return usecase.LLMOverviewResult{
		Categories:   decoded.Categories,
		Walkthroughs: decoded.Walkthroughs,
	}, nil
}
