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
var replyTaskPromptTemplateRaw string

// Config contains coding-agent runtime options.
type Config struct {
	Agent    string
	Provider string
	Model    string
}

// Answerer uses a coding agent for replycomment answers.
type Answerer struct {
	config Config
	logger usecase.Logger
}

type replyTaskPromptTemplateData struct {
	Repository    string
	RepoURL       string
	Base          string
	Head          string
	Title         string
	Description   string
	Question      string
	Thread        string
	ExtraGuidance string
}

// NewAnswerer creates a coding-agent replycomment adapter.
func NewAnswerer(config Config, logger usecase.Logger) (*Answerer, error) {
	if strings.TrimSpace(config.Agent) == "" {
		return nil, fmt.Errorf("coding agent is required")
	}
	if logger == nil {
		logger = stdlogger.Nop()
	}
	return &Answerer{config: config, logger: logger}, nil
}

// Answer generates an answer using the coding agent.
func (a *Answerer) Answer(ctx context.Context, payload usecase.ReplyCommentAnswerPayload) (string, error) {
	startedAt := time.Now()
	a.logger.Infof("Coding-agent replycomment started.")

	normalizedBase, normalizedHead := refs.NormalizePromptRefs(payload.Input.Base, payload.Input.Head)
	if payload.Environment == nil {
		return "", fmt.Errorf("code environment must not be nil")
	}
	if err := ensureDiffContentAvailable(ctx, payload.Environment, normalizedBase, normalizedHead); err != nil {
		return "", err
	}

	agent, err := payload.Environment.SetupAgent(ctx, domain.CodingAgentSetupOptions{
		Agent: a.config.Agent,
		Ref:   normalizedHead,
	})
	if err != nil {
		return "", fmt.Errorf("failed to setup coding agent: %w", err)
	}

	threadText := formatThread(payload.Thread)
	taskPrompt, err := sharedtext.RenderSimpleTemplate("reply_task_prompt", replyTaskPromptTemplateRaw, replyTaskPromptTemplateData{
		Repository:    payload.Input.Target.Repository,
		RepoURL:       payload.Input.RepoURL,
		Base:          normalizedBase,
		Head:          normalizedHead,
		Title:         payload.Input.Title,
		Description:   sharedtext.SingleLine(payload.Input.Description),
		Question:      payload.Question,
		Thread:        threadText,
		ExtraGuidance: strings.TrimSpace(payload.ExtraGuidance),
	})
	if err != nil {
		return "", err
	}

	rawText, err := runTask(ctx, agent, a.config, taskPrompt)
	if err != nil {
		return "", err
	}

	a.logger.Infof("Coding-agent replycomment completed.")
	a.logger.Debugf("Coding-agent replycomment took %d ms.", time.Since(startedAt).Milliseconds())
	return strings.TrimSpace(rawText), nil
}
