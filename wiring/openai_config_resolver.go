package wiring

import (
	"strings"

	"bentos-backend/config"
	sharedllm "bentos-backend/shared/llm"
)

// EffectiveOpenAIConfig contains the resolved OpenAI-compatible endpoint and model.
type EffectiveOpenAIConfig struct {
	BaseURL string
	Model   string
}

// ResolveEffectiveOpenAIConfig resolves base URL and model using config and optional CLI overrides.
func ResolveEffectiveOpenAIConfig(cfg config.Config, opts CLILLMOptions) (EffectiveOpenAIConfig, error) {
	baseURLInput := strings.TrimSpace(cfg.OpenAI.BaseURL)
	if strings.TrimSpace(opts.OpenAIBaseURL) != "" {
		baseURLInput = strings.TrimSpace(opts.OpenAIBaseURL)
	}

	resolvedBaseURL, resolvedModel, _, err := sharedllm.ResolveBaseURLAndModel(baseURLInput, cfg.OpenAI.Model, opts.OpenAIModel)
	if err != nil {
		return EffectiveOpenAIConfig{}, err
	}

	return EffectiveOpenAIConfig{
		BaseURL: resolvedBaseURL,
		Model:   resolvedModel,
	}, nil
}
