package tracing

import (
	"context"
	"encoding/json"
	"fmt"

	"bentos-backend/usecase"
	"bentos-backend/usecase/contracts"
)

// Generator wraps an LLM generator and emits trace logs for requests and outputs.
type Generator struct {
	base   contracts.LLMGenerator
	logger usecase.Logger
}

// NewGenerator creates a trace-logging LLM generator decorator.
func NewGenerator(base contracts.LLMGenerator, logger usecase.Logger) *Generator {
	return &Generator{
		base:   base,
		logger: logger,
	}
}

// Generate proxies text generation and logs input/output at trace level.
func (g *Generator) Generate(ctx context.Context, params contracts.GenerateParams) (string, error) {
	output, err := g.base.Generate(ctx, params)
	if err != nil {
		g.tracef("llm response method=Generate error=%q", err.Error())
		return "", err
	}

	g.tracef("llm response method=Generate bytes=%d output=%q", len(output), output)
	return output, nil
}

// GenerateJSON proxies JSON generation and logs input/output at trace level.
func (g *Generator) GenerateJSON(ctx context.Context, params contracts.GenerateParams, schema map[string]any) (map[string]any, error) {
	output, err := g.base.GenerateJSON(ctx, params, schema)
	if err != nil {
		g.tracef("llm response method=GenerateJSON error=%q", err.Error())
		return nil, err
	}

	compact := toCompactJSON(output)
	g.tracef("llm response method=GenerateJSON bytes=%d output=%s", len(compact), compact)
	return output, nil
}

func (g *Generator) tracef(format string, args ...any) {
	if g.logger == nil {
		return
	}
	g.logger.Tracef(format, args...)
}

func toCompactJSON(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf(`{"marshal_error":%q}`, err.Error())
	}
	return string(raw)
}
