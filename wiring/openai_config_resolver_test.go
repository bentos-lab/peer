package wiring

import (
	"testing"

	"bentos-backend/config"
	"github.com/stretchr/testify/require"
)

func TestResolveEffectiveOpenAIConfigUsesConfiguredModelForShortcut(t *testing.T) {
	effectiveConfig, err := ResolveEffectiveOpenAIConfig(config.Config{
		OpenAI: config.OpenAIConfig{
			BaseURL: "gemini",
			Model:   "gemini-3-pro-preview",
		},
	}, CLILLMOptions{})
	require.NoError(t, err)
	require.Equal(t, "https://generativelanguage.googleapis.com/v1beta/openai", effectiveConfig.BaseURL)
	require.Equal(t, "gemini-3-pro-preview", effectiveConfig.Model)
}

func TestResolveEffectiveOpenAIConfigUsesCLIModelOverride(t *testing.T) {
	effectiveConfig, err := ResolveEffectiveOpenAIConfig(config.Config{
		OpenAI: config.OpenAIConfig{
			BaseURL: "openai",
			Model:   "ignored-model",
		},
	}, CLILLMOptions{
		OpenAIModel: "gpt-4.1",
	})
	require.NoError(t, err)
	require.Equal(t, "https://api.openai.com/v1", effectiveConfig.BaseURL)
	require.Equal(t, "gpt-4.1", effectiveConfig.Model)
}

func TestResolveEffectiveOpenAIConfigRejectsFullURLWithoutModel(t *testing.T) {
	_, err := ResolveEffectiveOpenAIConfig(config.Config{
		OpenAI: config.OpenAIConfig{
			BaseURL: "https://example.com/v1",
			Model:   "",
		},
	}, CLILLMOptions{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "openai model is required when using full base URL")
}

func TestResolveEffectiveOpenAIConfigTrimsInputs(t *testing.T) {
	effectiveConfig, err := ResolveEffectiveOpenAIConfig(config.Config{
		OpenAI: config.OpenAIConfig{
			BaseURL: "  https://example.com/v1  ",
			Model:   "  config-model  ",
		},
	}, CLILLMOptions{
		OpenAIModel: "  flag-model  ",
	})
	require.NoError(t, err)
	require.Equal(t, "https://example.com/v1", effectiveConfig.BaseURL)
	require.Equal(t, "flag-model", effectiveConfig.Model)
}

func TestResolveEffectiveOpenAIConfigRejectsInvalidFullURL(t *testing.T) {
	_, err := ResolveEffectiveOpenAIConfig(config.Config{
		OpenAI: config.OpenAIConfig{
			BaseURL: "not-a-url",
			Model:   "custom-model",
		},
	}, CLILLMOptions{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "valid http(s) URL")
}
