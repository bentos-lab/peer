package codingagent

import (
	"context"
	"encoding/json"
	"testing"

	"bentos-backend/domain"
	"bentos-backend/usecase/contracts"
	"github.com/stretchr/testify/require"
)

type stubAgent struct {
	lastTask   string
	tasks      []string
	lastOpts   domain.CodingAgentRunOptions
	opts       []domain.CodingAgentRunOptions
	outputs    []string
	sessionIDs []string
	calls      int
	err        error
}

func (s *stubAgent) Run(_ context.Context, task string, opts domain.CodingAgentRunOptions) (domain.CodingAgentRunResult, error) {
	s.lastTask = task
	s.lastOpts = opts
	s.tasks = append(s.tasks, task)
	s.opts = append(s.opts, opts)
	s.calls++
	if s.err != nil {
		return domain.CodingAgentRunResult{}, s.err
	}
	if len(s.outputs) == 0 {
		return domain.CodingAgentRunResult{Text: "", SessionID: s.currentSessionID()}, nil
	}
	if s.calls > len(s.outputs) {
		return domain.CodingAgentRunResult{Text: s.outputs[len(s.outputs)-1], SessionID: s.currentSessionID()}, nil
	}
	return domain.CodingAgentRunResult{Text: s.outputs[s.calls-1], SessionID: s.currentSessionID()}, nil
}

func TestNewGenerator_ValidatesInputs(t *testing.T) {
	_, err := NewGenerator(nil, Config{Provider: "openai", Model: "model"}, nil)
	require.EqualError(t, err, "coding agent is required")
}

func TestGenerator_Generate_RendersTaskAndReturnsOutput(t *testing.T) {
	agent := &stubAgent{outputs: []string{"  done  "}}
	generator, err := NewGenerator(agent, Config{Provider: "openai", Model: "model"}, nil)
	require.NoError(t, err)

	result, err := generator.Generate(context.Background(), contracts.GenerateParams{
		SystemPrompt: "system",
		Messages:     []string{"hello", "world"},
	})
	require.NoError(t, err)
	require.Equal(t, "done", result)
	require.Contains(t, agent.lastTask, "System prompt:")
	require.Contains(t, agent.lastTask, "system")
	require.Contains(t, agent.lastTask, "- hello")
	require.Contains(t, agent.lastTask, "- world")
}

func TestGenerator_GenerateJSON_ParsesJSONOutput(t *testing.T) {
	agent := &stubAgent{outputs: []string{`{"summary":"done"}`}}
	generator, err := NewGenerator(agent, Config{Provider: "openai", Model: "model"}, nil)
	require.NoError(t, err)

	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"summary": map[string]any{"type": "string"},
		},
	}

	result, err := generator.GenerateJSON(context.Background(), contracts.GenerateParams{
		SystemPrompt: "system",
		Messages:     []string{"hello"},
	}, schema)
	require.NoError(t, err)
	require.Equal(t, "done", result["summary"])

	schemaRaw, err := json.Marshal(schema)
	require.NoError(t, err)
	require.Contains(t, agent.lastTask, string(schemaRaw))
}

func TestGenerator_GenerateJSON_RetriesOnInvalidJSON(t *testing.T) {
	agent := &stubAgent{outputs: []string{"not-json", "still-bad", `{"summary":"done"}`}}
	generator, err := NewGenerator(agent, Config{Provider: "openai", Model: "model"}, nil)
	require.NoError(t, err)

	result, err := generator.GenerateJSON(context.Background(), contracts.GenerateParams{
		SystemPrompt: "system",
		Messages:     []string{"hello"},
	}, nil)
	require.NoError(t, err)
	require.Equal(t, "done", result["summary"])
	require.Equal(t, 3, agent.calls)
}

func TestGenerator_GenerateJSON_RetriesWithSessionIDAndFixMessage(t *testing.T) {
	agent := &stubAgent{
		outputs:    []string{"not-json", `{"summary":"done"}`},
		sessionIDs: []string{"ses_1", "ses_1"},
	}
	generator, err := NewGenerator(agent, Config{Provider: "openai", Model: "model"}, nil)
	require.NoError(t, err)

	result, err := generator.GenerateJSON(context.Background(), contracts.GenerateParams{
		SystemPrompt: "system",
		Messages:     []string{"hello"},
	}, nil)
	require.NoError(t, err)
	require.Equal(t, "done", result["summary"])
	require.Equal(t, 2, agent.calls)
	require.Equal(t, "ses_1", agent.opts[1].SessionID)
	require.Contains(t, agent.tasks[1], "previous response was invalid")
	require.Contains(t, agent.tasks[1], "return JSON only")
}

func TestGenerator_GenerateJSON_RetriesOnSchemaValidationFailure(t *testing.T) {
	agent := &stubAgent{outputs: []string{`{"summary":123}`, `{"summary":"done"}`}}
	generator, err := NewGenerator(agent, Config{Provider: "openai", Model: "model"}, nil)
	require.NoError(t, err)

	schema := map[string]any{
		"type": "object",
		"required": []string{
			"summary",
		},
		"properties": map[string]any{
			"summary": map[string]any{"type": "string"},
		},
	}

	result, err := generator.GenerateJSON(context.Background(), contracts.GenerateParams{
		SystemPrompt: "system",
		Messages:     []string{"hello"},
	}, schema)
	require.NoError(t, err)
	require.Equal(t, "done", result["summary"])
	require.Equal(t, 2, agent.calls)
}

func TestGenerator_GenerateJSON_ReturnsSchemaValidationError(t *testing.T) {
	agent := &stubAgent{outputs: []string{`{"summary":123}`}}
	generator, err := NewGenerator(agent, Config{Provider: "openai", Model: "model"}, nil)
	require.NoError(t, err)

	schema := map[string]any{
		"type": "object",
		"required": []string{
			"summary",
		},
		"properties": map[string]any{
			"summary": map[string]any{"type": "string"},
		},
	}

	_, err = generator.GenerateJSON(context.Background(), contracts.GenerateParams{}, schema)
	require.Error(t, err)
	require.Equal(t, generateJSONMaxAttempts, agent.calls)
}

func TestGenerator_GenerateJSON_ReturnsDecodeError(t *testing.T) {
	agent := &stubAgent{outputs: []string{"not-json"}}
	generator, err := NewGenerator(agent, Config{Provider: "openai", Model: "model"}, nil)
	require.NoError(t, err)

	_, err = generator.GenerateJSON(context.Background(), contracts.GenerateParams{}, nil)
	require.Error(t, err)
	require.Equal(t, generateJSONMaxAttempts, agent.calls)
}
