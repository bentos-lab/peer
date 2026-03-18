package llm

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"

	"github.com/bentos-lab/peer/domain"
	"github.com/bentos-lab/peer/shared/logger/stdlogger"
	"github.com/bentos-lab/peer/usecase"
	"github.com/bentos-lab/peer/usecase/contracts"
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
		return usecase.LLMOverviewResult{}, fmt.Errorf("overview: render system prompt: %w", err)
	}

	userPrompt, err := renderOverviewUserPrompt(payload.Input, changedFiles)
	if err != nil {
		return usecase.LLMOverviewResult{}, fmt.Errorf("overview: render user prompt: %w", err)
	}

	outputMap, err := g.generator.GenerateJSON(ctx, contracts.GenerateParams{
		SystemPrompt: systemPrompt,
		Messages:     []string{userPrompt},
	}, overviewResponseSchema())
	if err != nil {
		return usecase.LLMOverviewResult{}, fmt.Errorf("overview: generate JSON output: %w", err)
	}

	raw, err := json.Marshal(outputMap)
	if err != nil {
		return usecase.LLMOverviewResult{}, fmt.Errorf("overview: encode model output: %w", err)
	}

	var decoded overviewModelOutput
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return usecase.LLMOverviewResult{}, fmt.Errorf("overview: invalid model output: %w", err)
	}

	if err := validateOverviewCategories(decoded.Categories); err != nil {
		return usecase.LLMOverviewResult{}, fmt.Errorf("overview: invalid category: %w", err)
	}

	g.logger.Debugf("The LLM overview generation produced %d categories and %d walkthrough groups.", len(decoded.Categories), len(decoded.Walkthroughs))

	return usecase.LLMOverviewResult{
		Categories:   decoded.Categories,
		Walkthroughs: decoded.Walkthroughs,
	}, nil
}
