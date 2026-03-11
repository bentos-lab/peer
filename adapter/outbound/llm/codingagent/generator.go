package codingagent

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"

	"bentos-backend/domain"
	"bentos-backend/shared/logger/stdlogger"
	"bentos-backend/usecase"
	"bentos-backend/usecase/contracts"
)

//go:embed task.md
var taskTemplateRaw string

//go:embed task_json.md
var taskJSONTemplateRaw string

const generateJSONMaxAttempts = 5

// Config contains coding-agent runtime options.
type Config struct {
	Provider string
	Model    string
}

// Generator runs LLM tasks through a coding agent.
type Generator struct {
	agent  contracts.CodingAgent
	config Config
	logger usecase.Logger
}

type taskTemplateData struct {
	SystemPrompt string
	Messages     []string
}

type taskJSONTemplateData struct {
	SystemPrompt string
	Messages     []string
	Schema       string
	HasSchema    bool
}

// NewGenerator creates a coding-agent LLM generator.
func NewGenerator(agent contracts.CodingAgent, config Config, logger usecase.Logger) (*Generator, error) {
	if agent == nil {
		return nil, fmt.Errorf("coding agent is required")
	}
	if logger == nil {
		logger = stdlogger.Nop()
	}
	return &Generator{
		agent:  agent,
		config: config,
		logger: logger,
	}, nil
}

// Generate runs a coding-agent task and returns raw text output.
func (g *Generator) Generate(ctx context.Context, params contracts.GenerateParams) (string, error) {
	task, err := renderTemplate("llm_task_prompt", taskTemplateRaw, taskTemplateData{
		SystemPrompt: strings.TrimSpace(params.SystemPrompt),
		Messages:     params.Messages,
	})
	if err != nil {
		return "", err
	}

	output, err := g.runTask(ctx, task)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output), nil
}

// GenerateJSON runs a coding-agent task and decodes JSON output.
func (g *Generator) GenerateJSON(ctx context.Context, params contracts.GenerateParams, schema map[string]any) (map[string]any, error) {
	schemaText, hasSchema, err := encodeSchema(schema)
	if err != nil {
		return nil, err
	}

	task, err := renderTemplate("llm_task_prompt_json", taskJSONTemplateRaw, taskJSONTemplateData{
		SystemPrompt: strings.TrimSpace(params.SystemPrompt),
		Messages:     params.Messages,
		Schema:       schemaText,
		HasSchema:    hasSchema,
	})
	if err != nil {
		return nil, err
	}

	var lastDecodeErr error
	for attempt := 1; attempt <= generateJSONMaxAttempts; attempt++ {
		output, err := g.runTask(ctx, task)
		if err != nil {
			return nil, err
		}
		output = strings.TrimRight(strings.TrimLeft(output, "```json"), "```")

		var decoded map[string]any
		if err := json.Unmarshal([]byte(output), &decoded); err == nil {
			return decoded, nil
		} else {
			lastDecodeErr = err
			g.logger.Warnf("coding-agent JSON decode failed (attempt %d/%d): %v", attempt, generateJSONMaxAttempts, err)
		}
	}

	return nil, lastDecodeErr
}

func (g *Generator) runTask(ctx context.Context, task string) (string, error) {
	task = strings.TrimSpace(task)
	if task == "" {
		return "", fmt.Errorf("task is required")
	}

	result, err := g.agent.Run(ctx, task, domain.CodingAgentRunOptions{
		Provider: g.config.Provider,
		Model:    g.config.Model,
	})
	if err != nil {
		return "", fmt.Errorf("failed to run coding agent task: %w", err)
	}
	return result.Text, nil
}

func renderTemplate(name string, templateRaw string, input any) (string, error) {
	parsedTemplate, err := template.New(name).Parse(templateRaw)
	if err != nil {
		return "", err
	}
	var rendered bytes.Buffer
	if err := parsedTemplate.Execute(&rendered, input); err != nil {
		return "", err
	}
	return rendered.String(), nil
}

func encodeSchema(schema map[string]any) (string, bool, error) {
	if len(schema) == 0 {
		return "", false, nil
	}
	raw, err := json.Marshal(schema)
	if err != nil {
		return "", false, err
	}
	return string(raw), true, nil
}
