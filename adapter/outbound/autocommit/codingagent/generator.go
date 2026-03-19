package codingagent

import (
	"context"
	_ "embed"
	"fmt"
	"strings"
	"time"

	"github.com/bentos-lab/peer/domain"
	"github.com/bentos-lab/peer/shared/logger/stdlogger"
	sharedtext "github.com/bentos-lab/peer/shared/text"
	"github.com/bentos-lab/peer/usecase"
)

//go:embed task.md
var commitTaskPromptTemplateRaw string

// Config contains coding-agent runtime options.
type Config struct {
	Agent    string
	Provider string
	Model    string
}

// Generator uses a coding agent to build conventional commit messages.
type Generator struct {
	config Config
	logger usecase.Logger
}

type commitTaskPromptTemplateData struct {
	Repository  string
	RepoURL     string
	Staged      bool
	Title       string
	Description string
}

// NewGenerator creates a coding-agent commit message generator.
func NewGenerator(config Config, logger usecase.Logger) (*Generator, error) {
	if strings.TrimSpace(config.Agent) == "" {
		return nil, fmt.Errorf("coding agent is required")
	}
	if logger == nil {
		logger = stdlogger.Nop()
	}
	return &Generator{config: config, logger: logger}, nil
}

// GenerateCommitMessage produces a conventional commit message from changed content.
func (g *Generator) GenerateCommitMessage(ctx context.Context, payload usecase.CommitMessagePayload) (string, error) {
	startedAt := time.Now()
	g.logger.Infof("Coding-agent commit message generation started.")

	headToken := "@all"
	if payload.Staged {
		headToken = "@staged"
	}

	if payload.Environment == nil {
		return "", fmt.Errorf("code environment must not be nil")
	}
	if err := ensureDiffContentAvailable(ctx, payload.Environment, "", headToken); err != nil {
		return "", err
	}

	agent, err := payload.Environment.SetupAgent(ctx, domain.CodingAgentSetupOptions{
		Agent: g.config.Agent,
		Ref:   headToken,
	})
	if err != nil {
		return "", fmt.Errorf("failed to setup coding agent: %w", err)
	}

	taskPrompt, err := sharedtext.RenderSimpleTemplate("commit_task_prompt", commitTaskPromptTemplateRaw, commitTaskPromptTemplateData{
		Repository:  payload.Input.Target.Repository,
		RepoURL:     payload.Input.RepoURL,
		Staged:      payload.Staged,
		Title:       payload.Input.Title,
		Description: sharedtext.SingleLine(payload.Input.Description),
	})
	if err != nil {
		return "", err
	}

	output, err := runTask(ctx, agent, g.config, taskPrompt)
	if err != nil {
		return "", err
	}

	g.logger.Infof("Coding-agent commit message generation completed.")
	g.logger.Debugf("Coding-agent commit message generation took %d ms.", time.Since(startedAt).Milliseconds())
	return strings.TrimSpace(output), nil
}
