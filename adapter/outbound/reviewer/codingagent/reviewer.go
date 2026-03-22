package codingagent

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/bentos-lab/peer/domain"
	"github.com/bentos-lab/peer/shared/logger/stdlogger"
	"github.com/bentos-lab/peer/shared/refs"
	sharedtext "github.com/bentos-lab/peer/shared/text"
	"github.com/bentos-lab/peer/usecase"
	"github.com/bentos-lab/peer/usecase/contracts"
)

//go:embed task.md
var reviewTaskPromptTemplateRaw string

//go:embed formatting_system.md
var reviewFormattingSystemPrompt string

// Config contains coding-agent runtime options.
type Config struct {
	Agent    string
	Provider string
	Model    string
}

// Reviewer uses a coding agent for analysis and an LLM for JSON formatting.
type Reviewer struct {
	formatter contracts.LLMGenerator
	config    Config
	logger    usecase.Logger
}

type reviewModelOutput struct {
	Summary  string            `json:"summary"`
	Findings []json.RawMessage `json:"findings"`
}

type reviewTaskPromptTemplateData struct {
	Repository    string
	RepoURL       string
	Base          string
	Head          string
	Title         string
	Description   string
	Language      string
	Suggestions   bool
	RulePackText  string
	CustomRuleset string
}

// NewReviewer creates a coding-agent reviewer adapter.
func NewReviewer(formatter contracts.LLMGenerator, config Config, logger usecase.Logger) (*Reviewer, error) {
	if formatter == nil {
		return nil, fmt.Errorf("formatter llm generator must not be nil")
	}
	if strings.TrimSpace(config.Agent) == "" {
		return nil, fmt.Errorf("coding agent is required")
	}
	if logger == nil {
		logger = stdlogger.Nop()
	}
	return &Reviewer{
		formatter: formatter,
		config:    config,
		logger:    logger,
	}, nil
}

// Review generates findings using coding-agent analysis and formatter conversion.
func (r *Reviewer) Review(ctx context.Context, payload usecase.LLMReviewPayload) (usecase.LLMReviewResult, error) {
	startedAt := time.Now()
	r.logger.Infof("Coding-agent review started.")

	normalizedBase, normalizedHead := refs.NormalizePromptRefs(payload.Input.Base, payload.Input.Head)

	if payload.Environment == nil {
		return usecase.LLMReviewResult{}, fmt.Errorf("code environment must not be nil")
	}
	if err := payload.Environment.EnsureDiffContentAvailable(ctx, domain.CodeEnvironmentLoadOptions{
		Base: normalizedBase,
		Head: normalizedHead,
	}); err != nil {
		return usecase.LLMReviewResult{}, err
	}

	agent, err := r.setupAgent(ctx, payload.Environment, normalizedHead)
	if err != nil {
		return usecase.LLMReviewResult{}, err
	}
	taskPrompt, err := sharedtext.RenderSimpleTemplate("review_task_prompt", reviewTaskPromptTemplateRaw, reviewTaskPromptTemplateData{
		Repository:    payload.Input.Target.Repository,
		RepoURL:       payload.Input.RepoURL,
		Base:          normalizedBase,
		Head:          normalizedHead,
		Title:         payload.Input.Title,
		Description:   sharedtext.SingleLine(payload.Input.Description),
		Language:      resolveLanguage(payload.Input.Language),
		Suggestions:   payload.Suggestions,
		RulePackText:  strings.Join(payload.RulePack.Instructions, "\n\n"),
		CustomRuleset: strings.TrimSpace(payload.CustomRuleset),
	})
	if err != nil {
		return usecase.LLMReviewResult{}, err
	}

	result, err := agent.Run(ctx, strings.TrimSpace(taskPrompt), domain.CodingAgentRunOptions{
		Provider: r.config.Provider,
		Model:    r.config.Model,
	})
	if err != nil {
		return usecase.LLMReviewResult{}, fmt.Errorf("failed to run coding agent task: %w", err)
	}
	rawText := strings.TrimSpace(result.Text)

	outputMap, err := r.formatter.GenerateJSON(ctx, contracts.GenerateParams{
		SystemPrompt: reviewFormattingSystemPrompt,
		Messages:     []string{rawText},
	}, reviewResponseSchema())
	if err != nil {
		return usecase.LLMReviewResult{}, err
	}

	var decoded reviewModelOutput
	if err := decodeModelOutput(outputMap, &decoded); err != nil {
		return usecase.LLMReviewResult{}, err
	}

	findings := make([]domain.Finding, 0, len(decoded.Findings))
	for _, findingRaw := range decoded.Findings {
		var finding domain.Finding
		if err := json.Unmarshal(findingRaw, &finding); err != nil {
			return usecase.LLMReviewResult{}, fmt.Errorf("invalid finding format: %w", err)
		}
		if finding.StartLine <= 0 || finding.EndLine <= 0 || finding.StartLine > finding.EndLine {
			return usecase.LLMReviewResult{}, fmt.Errorf("invalid finding range for %q: %d-%d", finding.FilePath, finding.StartLine, finding.EndLine)
		}
		finding.SuggestedChange = normalizeSuggestedChange(finding.SuggestedChange)
		findings = append(findings, finding)
	}

	r.logger.Infof("Coding-agent review completed.")
	r.logger.Debugf("Coding-agent review took %d ms and produced %d findings.", time.Since(startedAt).Milliseconds(), len(findings))
	return usecase.LLMReviewResult{Summary: decoded.Summary, Findings: findings}, nil
}
