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

func (g *Generator) runTask(ctx context.Context, task string, opts domain.CodingAgentRunOptions) (domain.CodingAgentRunResult, error) {
	task = strings.TrimSpace(task)
	if task == "" {
		return domain.CodingAgentRunResult{}, fmt.Errorf("task is required")
	}

	result, err := g.agent.Run(ctx, task, opts)
	if err != nil {
		return domain.CodingAgentRunResult{}, fmt.Errorf("failed to run coding agent task: %w", err)
	}
	return result, nil
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

func compileSchema(schemaText string) (*jsonschema.Schema, error) {
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("schema.json", strings.NewReader(schemaText)); err != nil {
		return nil, err
	}
	return compiler.Compile("schema.json")
}

func buildJSONFixMessage(validationErr error, lastOutput string) string {
	message := fmt.Sprintf("Your previous response was invalid JSON or did not match the schema: %v.", validationErr)
	if strings.TrimSpace(lastOutput) != "" {
		message = fmt.Sprintf("%s Invalid output was: %s", message, truncateRetryOutput(lastOutput, 500))
	}
	return message + " Please fix the output and return JSON only that strictly matches the schema."
}

func truncateRetryOutput(output string, maxChars int) string {
	if maxChars <= 0 || len(output) <= maxChars {
		return output
	}
	return output[:maxChars] + "...(truncated)"
}
