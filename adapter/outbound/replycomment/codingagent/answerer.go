package codingagent

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"strings"
	"text/template"
	"time"

	"bentos-backend/domain"
	"bentos-backend/shared/logger/stdlogger"
	sharedtext "bentos-backend/shared/text"
	"bentos-backend/usecase"
	uccontracts "bentos-backend/usecase/contracts"
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

	normalizedBase, normalizedHead := normalizePromptRefs(payload.Input.Base, payload.Input.Head)
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
	taskPrompt, err := renderSimpleTemplate("reply_task_prompt", replyTaskPromptTemplateRaw, replyTaskPromptTemplateData{
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

func normalizePromptRefs(base string, head string) (string, string) {
	normalizedBase := strings.TrimSpace(base)
	normalizedHead := strings.TrimSpace(head)
	if normalizedHead == "@staged" || normalizedHead == "@all" {
		return "", normalizedHead
	}
	return normalizedBase, normalizedHead
}

func ensureDiffContentAvailable(ctx context.Context, environment uccontracts.CodeEnvironment, base string, head string) error {
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

func runTask(ctx context.Context, agent uccontracts.CodingAgent, cfg Config, task string) (string, error) {
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

func formatThread(thread domain.CommentThread) string {
	if len(thread.Context) == 0 && len(thread.Comments) == 0 {
		return "(no prior comments)"
	}
	var builder strings.Builder
	if len(thread.Context) > 0 {
		builder.WriteString("Parent context:\n")
		for _, line := range thread.Context {
			line = strings.TrimRight(line, " ")
			if line == "" {
				builder.WriteString("\n")
				continue
			}
			builder.WriteString(line)
			builder.WriteString("\n")
		}
		builder.WriteString("\n")
	}
	for _, comment := range thread.Comments {
		author := strings.TrimSpace(comment.Author.Login)
		if author == "" {
			author = "unknown"
		}
		builder.WriteString(fmt.Sprintf("- [%s] %s\n", comment.CreatedAt.Format(time.RFC3339), author))
		body := strings.TrimSpace(comment.Body)
		if body == "" {
			body = "(empty)"
		}
		for _, line := range strings.Split(body, "\n") {
			builder.WriteString(fmt.Sprintf("  %s\n", line))
		}
	}
	return strings.TrimSpace(builder.String())
}
