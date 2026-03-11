package wiring

import (
	"testing"

	"bentos-backend/config"
	"github.com/stretchr/testify/require"
)

func TestResolveLLMSelectionUsesCodingAgentWhenBaseURLEmpty(t *testing.T) {
	selection, err := ResolveLLMSelection(config.Config{
		OpenAI: config.OpenAIConfig{
			BaseURL: "",
		},
	}, CLILLMOptions{})
	require.NoError(t, err)
	require.False(t, selection.UseOpenAI)
}

func TestResolveLLMSelectionUsesOpenAIWhenBaseURLPresent(t *testing.T) {
	selection, err := ResolveLLMSelection(config.Config{
		OpenAI: config.OpenAIConfig{
			BaseURL: "openai",
			Model:   "",
			APIKey:  "key",
		},
	}, CLILLMOptions{})
	require.NoError(t, err)
	require.True(t, selection.UseOpenAI)
	require.Equal(t, "https://api.openai.com/v1", selection.OpenAI.BaseURL)
	require.Equal(t, "gpt-4.1-mini", selection.OpenAI.Model)
}

func TestResolveLLMSelectionAllowsEmptyBaseURLOverride(t *testing.T) {
	selection, err := ResolveLLMSelection(config.Config{
		OpenAI: config.OpenAIConfig{
			BaseURL: "openai",
			Model:   "gpt-4.1-mini",
			APIKey:  "key",
		},
	}, CLILLMOptions{
		OpenAIBaseURLSet: true,
		OpenAIBaseURL:    "",
	})
	require.NoError(t, err)
	require.False(t, selection.UseOpenAI)
}

func TestResolveLLMSelectionRejectsMissingAPIKey(t *testing.T) {
	_, err := ResolveLLMSelection(config.Config{
		OpenAI: config.OpenAIConfig{
			BaseURL: "openai",
			Model:   "gpt-4.1-mini",
		},
	}, CLILLMOptions{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "openai API key is required")
}
