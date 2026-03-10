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
	sharedtext "bentos-backend/shared/text"
	"bentos-backend/shared/logger/stdlogger"
	"bentos-backend/usecase"
	uccontracts "bentos-backend/usecase/contracts"
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
	Repository  string
	RepoURL     string
	Base        string
	Head        string
	Title       string
	Description string
	Docs        bool
	Tests       bool
	HeadBranch  string
}

// NewGenerator creates a coding-agent autogen adapter.
func NewGenerator(config Config, logger usecase.Logger) (*Generator, error) {
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
	return &Generator{config: config, logger: logger}, nil
}

// Generate applies missing tests/docs/comments using the coding agent.
func (g *Generator) Generate(ctx context.Context, payload usecase.AutogenPayload) (string, error) {
	startedAt := time.Now()
	g.logger.Infof("Coding-agent autogen started.")

	if payload.Environment == nil {
		return "", fmt.Errorf("code environment must not be nil")
	}

	normalizedBase, normalizedHead := normalizePromptRefs(payload.Input.Base, payload.Input.Head)
	if err := ensureDiffContentAvailable(ctx, payload.Environment, normalizedBase, normalizedHead); err != nil {
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

	taskPrompt, err := renderSimpleTemplate("autogen_task_prompt", autogenTaskPromptTemplateRaw, autogenTaskPromptTemplateData{
		Repository:  payload.Input.Target.Repository,
		RepoURL:     payload.Input.RepoURL,
		Base:        normalizedBase,
		Head:        normalizedHead,
		Title:       payload.Input.Title,
		Description: sharedtext.SingleLine(payload.Input.Description),
		Docs:        payload.Docs,
		Tests:       payload.Tests,
		HeadBranch:  strings.TrimSpace(payload.HeadBranch),
	})
	if err != nil {
		return "", err
	}

	output, err := runTask(ctx, agent, g.config, taskPrompt)
	if err != nil {
		return "", err
	}

	g.logger.Infof("Coding-agent autogen completed.")
	g.logger.Debugf("Coding-agent autogen took %d ms.", time.Since(startedAt).Milliseconds())
	return output, nil
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
