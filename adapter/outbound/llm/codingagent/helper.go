package codingagent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"

	"bentos-backend/domain"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

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
