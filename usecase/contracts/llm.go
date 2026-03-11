package contracts

import "context"

// GenerateParams defines generic generation inputs for an LLM provider.
type GenerateParams struct {
	SystemPrompt string
	Messages     []string
}

// LLMGenerator is the generic LLM contract for outbound providers.
type LLMGenerator interface {
	Generate(ctx context.Context, params GenerateParams) (string, error)
	GenerateJSON(ctx context.Context, params GenerateParams, schema map[string]any) (map[string]any, error)
}
