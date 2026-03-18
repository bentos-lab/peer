package contracts

import "context"

// Message represents one chat message passed to an LLM.
type Message struct {
	Role    string
	Content string
}

// GenerateParams defines generic generation inputs for an LLM provider.
type GenerateParams struct {
	SystemPrompt   string
	Messages       []Message
	Temperature    *float64
	MaxTokens      *int
	Tools          []map[string]any
	ResponseSchema map[string]any
	Metadata       map[string]string
}

// LLMGenerator is the generic LLM contract for outbound providers.
type LLMGenerator interface {
	Generate(ctx context.Context, params GenerateParams) (string, error)
	GenerateJSON(ctx context.Context, params GenerateParams) (map[string]any, error)
}
