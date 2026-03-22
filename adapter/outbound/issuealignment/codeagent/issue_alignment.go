package codeagent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bentos-lab/peer/domain"
	"github.com/bentos-lab/peer/shared/logger/stdlogger"
	"github.com/bentos-lab/peer/shared/refs"
	"github.com/bentos-lab/peer/usecase"
	"github.com/bentos-lab/peer/usecase/contracts"

	_ "embed"
)

//go:embed task.md
var issueAlignmentTaskTemplateRaw string

//go:embed key_ideas_prompt.md
var issueAlignmentKeyIdeasPromptRaw string

//go:embed formatting_system.md
var issueAlignmentFormattingSystemPrompt string

type issueAlignmentKeyIdeasOutput struct {
	KeyIdeas []string `json:"keyIdeas"`
}

type issueAlignmentModelOutput struct {
	Issue        domain.IssueReference              `json:"issue"`
	KeyIdeas     []string                           `json:"keyIdeas"`
	Requirements []domain.IssueAlignmentRequirement `json:"requirements"`
}

// Config contains coding-agent runtime options.
type Config struct {
	Agent    string
	Provider string
	Model    string
}

// IssueAlignmentGenerator implements usecase.IssueAlignmentGenerator via a coding agent and LLM formatter.
type IssueAlignmentGenerator struct {
	formatter contracts.LLMGenerator
	config    Config
	logger    usecase.Logger
}

// NewIssueAlignmentGenerator creates an issue alignment generator backed by a coding agent and LLM formatter.
func NewIssueAlignmentGenerator(formatter contracts.LLMGenerator, config Config, logger usecase.Logger) (*IssueAlignmentGenerator, error) {
	if formatter == nil {
		return nil, fmt.Errorf("formatter llm generator must not be nil")
	}
	if strings.TrimSpace(config.Agent) == "" {
		return nil, fmt.Errorf("coding agent is required")
	}
	if logger == nil {
		logger = stdlogger.Nop()
	}
	return &IssueAlignmentGenerator{formatter: formatter, config: config, logger: logger}, nil
}

// GenerateIssueAlignment creates issue alignment output from changed content and linked issues.
func (g *IssueAlignmentGenerator) GenerateIssueAlignment(ctx context.Context, payload usecase.LLMIssueAlignmentPayload) (domain.IssueAlignmentResult, error) {
	if payload.Environment == nil {
		return domain.IssueAlignmentResult{}, fmt.Errorf("code environment must not be nil")
	}
	if len(payload.IssueAlignment.Candidates) == 0 {
		return domain.IssueAlignmentResult{}, fmt.Errorf("issue alignment requires issue candidates")
	}

	normalizedBase, normalizedHead := refs.NormalizePromptRefs(payload.Input.Base, payload.Input.Head)

	changedFiles, err := payload.Environment.LoadChangedFiles(ctx, domain.CodeEnvironmentLoadOptions{
		Base: normalizedBase,
		Head: normalizedHead,
	})
	if err != nil {
		return domain.IssueAlignmentResult{}, err
	}
	g.logger.Debugf("The issue alignment input includes %d changed files.", len(changedFiles))

	keyIdeasPrompt, err := renderIssueKeyIdeasPrompt(payload.IssueAlignment.Candidates, issueAlignmentKeyIdeasPromptRaw)
	if err != nil {
		return domain.IssueAlignmentResult{}, fmt.Errorf("issue alignment: render key ideas prompt: %w", err)
	}

	keyIdeasMap, err := g.formatter.GenerateJSON(ctx, contracts.GenerateParams{
		SystemPrompt: issueKeyIdeasSystemPrompt,
		Messages:     []string{keyIdeasPrompt},
	}, issueKeyIdeasSchema())
	if err != nil {
		return domain.IssueAlignmentResult{}, fmt.Errorf("issue alignment: generate key ideas: %w", err)
	}

	keyIdeasRaw, err := json.Marshal(keyIdeasMap)
	if err != nil {
		return domain.IssueAlignmentResult{}, fmt.Errorf("issue alignment: encode key ideas: %w", err)
	}

	var keyIdeasOutput issueAlignmentKeyIdeasOutput
	if err := json.Unmarshal(keyIdeasRaw, &keyIdeasOutput); err != nil {
		return domain.IssueAlignmentResult{}, fmt.Errorf("issue alignment: invalid key ideas output: %w", err)
	}

	keyIdeas := normalizeKeyIdeas(keyIdeasOutput.KeyIdeas)
	if len(keyIdeas) == 0 {
		return domain.IssueAlignmentResult{}, fmt.Errorf("issue alignment: no key ideas generated")
	}

	taskPrompt, err := renderIssueAlignmentTask(payload, keyIdeas, changedFiles, issueAlignmentTaskTemplateRaw)
	if err != nil {
		return domain.IssueAlignmentResult{}, fmt.Errorf("issue alignment: render task prompt: %w", err)
	}

	agent, err := payload.Environment.SetupAgent(ctx, domain.CodingAgentSetupOptions{
		Agent: g.config.Agent,
		Ref:   normalizedHead,
	})
	if err != nil {
		return domain.IssueAlignmentResult{}, fmt.Errorf("failed to setup coding agent: %w", err)
	}

	result, err := agent.Run(ctx, strings.TrimSpace(taskPrompt), domain.CodingAgentRunOptions{
		Provider: g.config.Provider,
		Model:    g.config.Model,
	})
	if err != nil {
		return domain.IssueAlignmentResult{}, fmt.Errorf("failed to run coding agent task: %w", err)
	}
	rawText := strings.TrimSpace(result.Text)

	outputMap, err := g.formatter.GenerateJSON(ctx, contracts.GenerateParams{
		SystemPrompt: issueAlignmentFormattingSystemPrompt,
		Messages:     []string{strings.TrimSpace(rawText)},
	}, issueAlignmentResponseSchema())
	if err != nil {
		return domain.IssueAlignmentResult{}, fmt.Errorf("issue alignment: generate JSON output: %w", err)
	}

	encoded, err := json.Marshal(outputMap)
	if err != nil {
		return domain.IssueAlignmentResult{}, fmt.Errorf("issue alignment: encode JSON output: %w", err)
	}

	var decoded issueAlignmentModelOutput
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		return domain.IssueAlignmentResult{}, fmt.Errorf("issue alignment: invalid JSON output: %w", err)
	}

	alignment := domain.IssueAlignmentResult{
		Issue:        decoded.Issue,
		KeyIdeas:     keyIdeas,
		Requirements: normalizeIssueAlignmentRequirements(decoded.Requirements),
	}

	fallback := fallbackIssueReference(payload.IssueAlignment.Candidates)
	if alignment.Issue.Number == 0 && fallback.Number != 0 {
		alignment.Issue = fallback
	}
	if alignment.Issue.Repository == "" && fallback.Repository != "" {
		alignment.Issue.Repository = fallback.Repository
	}
	if alignment.Issue.Title == "" && fallback.Title != "" {
		alignment.Issue.Title = fallback.Title
	}

	return alignment, nil
}

func normalizeIssueAlignmentRequirements(requirements []domain.IssueAlignmentRequirement) []domain.IssueAlignmentRequirement {
	if len(requirements) == 0 {
		return nil
	}
	filtered := make([]domain.IssueAlignmentRequirement, 0, len(requirements))
	for _, requirement := range requirements {
		trimmed := strings.TrimSpace(requirement.Requirement)
		coverage := strings.TrimSpace(requirement.Coverage)
		if trimmed == "" || coverage == "" {
			continue
		}
		filtered = append(filtered, domain.IssueAlignmentRequirement{Requirement: trimmed, Coverage: coverage})
	}
	return filtered
}
