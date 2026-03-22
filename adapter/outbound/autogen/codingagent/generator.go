package codingagent

import (
	"context"
	_ "embed"
	"fmt"
	"strings"
	"time"

	"github.com/bentos-lab/peer/domain"
	"github.com/bentos-lab/peer/shared/logger/stdlogger"
	"github.com/bentos-lab/peer/shared/refs"
	sharedtext "github.com/bentos-lab/peer/shared/text"
	"github.com/bentos-lab/peer/usecase"
)

//go:embed task.md
var autogenTaskPromptTemplateRaw string

// Config contains coding-agent runtime options.
type Config struct {
	Agent    string
	Provider string
	Model    string
}

// Generator uses a coding agent to apply missing tests/docs/comments.
type Generator struct {
	config Config
	logger usecase.Logger
}

type autogenTaskPromptTemplateData struct {
	Repository    string
	RepoURL       string
	Base          string
	Head          string
	Title         string
	Description   string
	Docs          bool
	Tests         bool
	HeadBranch    string
	ExtraGuidance string
}

// NewGenerator creates a coding-agent autogen adapter.
func NewGenerator(config Config, logger usecase.Logger) (*Generator, error) {
	if strings.TrimSpace(config.Agent) == "" {
		return nil, fmt.Errorf("coding agent is required")
	}
	if logger == nil {
		logger = stdlogger.Nop()
	}
	return &Generator{config: config, logger: logger}, nil
}

// Generate applies missing tests/docs/comments using the coding agent.
func (g *Generator) Generate(ctx context.Context, payload usecase.AutogenPayload) (string, error) {
	startedAt := time.Now()
	g.logger.Infof("Coding-agent autogen started.")

	if payload.Environment == nil {
		return "", fmt.Errorf("code environment must not be nil")
	}

	normalizedBase, normalizedHead := refs.NormalizePromptRefs(payload.Input.Base, payload.Input.Head)
	if err := payload.Environment.EnsureDiffContentAvailable(ctx, domain.CodeEnvironmentLoadOptions{
		Base: normalizedBase,
		Head: normalizedHead,
	}); err != nil {
		return "", err
	}

	headRef := strings.TrimSpace(payload.HeadBranch)
	if headRef == "" {
		headRef = normalizedHead
	}
	agent, err := payload.Environment.SetupAgent(ctx, domain.CodingAgentSetupOptions{
		Agent: g.config.Agent,
		Ref:   headRef,
	})
	if err != nil {
		return "", fmt.Errorf("failed to setup coding agent: %w", err)
	}

	taskPrompt, err := sharedtext.RenderSimpleTemplate("autogen_task_prompt", autogenTaskPromptTemplateRaw, autogenTaskPromptTemplateData{
		Repository:    payload.Input.Target.Repository,
		RepoURL:       payload.Input.RepoURL,
		Base:          normalizedBase,
		Head:          normalizedHead,
		Title:         payload.Input.Title,
		Description:   sharedtext.SingleLine(payload.Input.Description),
		Docs:          payload.Docs,
		Tests:         payload.Tests,
		HeadBranch:    strings.TrimSpace(payload.HeadBranch),
		ExtraGuidance: strings.TrimSpace(payload.ExtraGuidance),
	})
	if err != nil {
		return "", err
	}

	result, err := agent.Run(ctx, strings.TrimSpace(taskPrompt), domain.CodingAgentRunOptions{
		Provider: g.config.Provider,
		Model:    g.config.Model,
	})
	if err != nil {
		return "", fmt.Errorf("failed to run coding agent task: %w", err)
	}
	output := strings.TrimSpace(result.Text)

	g.logger.Infof("Coding-agent autogen completed.")
	g.logger.Debugf("Coding-agent autogen took %d ms.", time.Since(startedAt).Milliseconds())
	return output, nil
}
