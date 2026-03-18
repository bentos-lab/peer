package codingagent

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bentos-lab/peer/domain"
	"github.com/bentos-lab/peer/shared/logger/stdlogger"
	"github.com/bentos-lab/peer/usecase"
	"github.com/bentos-lab/peer/usecase/contracts"

	"github.com/santhosh-tekuri/jsonschema/v5"
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

	output, err := g.runTask(ctx, task, domain.CodingAgentRunOptions{
		Provider: g.config.Provider,
		Model:    g.config.Model,
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output.Text), nil
}

// GenerateJSON runs a coding-agent task and decodes JSON output.
func (g *Generator) GenerateJSON(ctx context.Context, params contracts.GenerateParams, schema map[string]any) (map[string]any, error) {
	schemaText, hasSchema, err := encodeSchema(schema)
	if err != nil {
		return nil, err
	}
	var compiledSchema *jsonschema.Schema
	if hasSchema {
		compiledSchema, err = compileSchema(schemaText)
		if err != nil {
			return nil, err
		}
	}

	baseMessages := append([]string(nil), params.Messages...)
	var lastErr error
	var lastOutput string
	sessionID := ""
	for attempt := 1; attempt <= generateJSONMaxAttempts; attempt++ {
		taskMessages := baseMessages
		if attempt > 1 && lastErr != nil {
			taskMessages = append(taskMessages, buildJSONFixMessage(lastErr, lastOutput))
		}
		task, err := renderTemplate("llm_task_prompt_json", taskJSONTemplateRaw, taskJSONTemplateData{
			SystemPrompt: strings.TrimSpace(params.SystemPrompt),
			Messages:     taskMessages,
			Schema:       schemaText,
			HasSchema:    hasSchema,
		})
		if err != nil {
			return nil, err
		}

		result, err := g.runTask(ctx, task, domain.CodingAgentRunOptions{
			Provider:  g.config.Provider,
			Model:     g.config.Model,
			SessionID: sessionID,
		})
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(result.SessionID) != "" {
			sessionID = strings.TrimSpace(result.SessionID)
		}
		output := strings.TrimRight(strings.TrimLeft(result.Text, "```json"), "```")
		lastOutput = output

		var decoded map[string]any
		if err := json.Unmarshal([]byte(output), &decoded); err != nil {
			lastErr = err
			g.logger.Warnf("coding-agent JSON decode failed (attempt %d/%d): %v", attempt, generateJSONMaxAttempts, err)
			continue
		}
		if compiledSchema != nil {
			if err := compiledSchema.Validate(decoded); err != nil {
				lastErr = err
				g.logger.Warnf("coding-agent JSON schema validation failed (attempt %d/%d): %v", attempt, generateJSONMaxAttempts, err)
				continue
			}
		}
		return decoded, nil
	}

	return nil, lastErr
}
