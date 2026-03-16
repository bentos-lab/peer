package wiring

import (
	"fmt"
	"strings"

	"bentos-backend/config"
	sharedllm "bentos-backend/shared/llm"
)

// CLILLMOptions contains CLI-only LLM overrides.
type CLILLMOptions struct {
	OpenAIBaseURL        string
	OpenAIBaseURLSet     bool
	OpenAIModel          string
	OpenAIAPIKey         string
	OpenAIAPIKeySet      bool
	CodeAgent            string
	CodeAgentSet         bool
	CodeAgentProvider    string
	CodeAgentProviderSet bool
	CodeAgentModel       string
	CodeAgentModelSet    bool
	ForceCLIPublishers   bool
	VCSProvider          string
	VCSHost              string
}

// EffectiveOpenAIConfig contains the resolved OpenAI-compatible endpoint and model.
type EffectiveOpenAIConfig struct {
	BaseURL string
	Model   string
}

// LLMSelection represents the resolved LLM mode for formatting/sanitization.
type LLMSelection struct {
	UseOpenAI    bool
	OpenAI       EffectiveOpenAIConfig
	OpenAIAPIKey string
}

// ResolveEffectiveOpenAIConfig resolves base URL and model using explicit inputs.
func ResolveEffectiveOpenAIConfig(baseURLInput, configModel, flagModel string) (EffectiveOpenAIConfig, error) {
	resolvedBaseURL, resolvedModel, _, err := sharedllm.ResolveBaseURLAndModel(baseURLInput, configModel, flagModel)
	if err != nil {
		return EffectiveOpenAIConfig{}, err
	}
	return EffectiveOpenAIConfig{
		BaseURL: resolvedBaseURL,
		Model:   resolvedModel,
	}, nil
}

// ResolveLLMSelection resolves whether OpenAI or coding-agent LLM should be used.
func ResolveLLMSelection(cfg config.Config, opts CLILLMOptions) (LLMSelection, error) {
	baseURLInput := strings.TrimSpace(cfg.OpenAI.BaseURL)
	if opts.OpenAIBaseURLSet {
		baseURLInput = strings.TrimSpace(opts.OpenAIBaseURL)
	}
	if baseURLInput == "" {
		return LLMSelection{UseOpenAI: false}, nil
	}

	effectiveConfig, err := ResolveEffectiveOpenAIConfig(baseURLInput, cfg.OpenAI.Model, opts.OpenAIModel)
	if err != nil {
		return LLMSelection{}, err
	}

	apiKey := strings.TrimSpace(cfg.OpenAI.APIKey)
	if opts.OpenAIAPIKeySet {
		apiKey = strings.TrimSpace(opts.OpenAIAPIKey)
	}
	if apiKey == "" {
		return LLMSelection{}, fmt.Errorf("openai API key is required")
	}

	return LLMSelection{
		UseOpenAI:    true,
		OpenAI:       effectiveConfig,
		OpenAIAPIKey: apiKey,
	}, nil
}
