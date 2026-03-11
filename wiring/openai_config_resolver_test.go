package wiring

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveEffectiveOpenAIConfigUsesConfiguredModelForShortcut(t *testing.T) {
	effectiveConfig, err := ResolveEffectiveOpenAIConfig("gemini", "gemini-3-pro-preview", "")
	require.NoError(t, err)
	require.Equal(t, "https://generativelanguage.googleapis.com/v1beta/openai", effectiveConfig.BaseURL)
	require.Equal(t, "gemini-3-pro-preview", effectiveConfig.Model)
}

func TestResolveEffectiveOpenAIConfigUsesCLIModelOverride(t *testing.T) {
	effectiveConfig, err := ResolveEffectiveOpenAIConfig("openai", "ignored-model", "gpt-4.1")
	require.NoError(t, err)
	require.Equal(t, "https://api.openai.com/v1", effectiveConfig.BaseURL)
	require.Equal(t, "gpt-4.1", effectiveConfig.Model)
}

func TestResolveEffectiveOpenAIConfigRejectsFullURLWithoutModel(t *testing.T) {
	_, err := ResolveEffectiveOpenAIConfig("https://example.com/v1", "", "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "openai model is required when using full base URL")
}

func TestResolveEffectiveOpenAIConfigTrimsInputs(t *testing.T) {
	effectiveConfig, err := ResolveEffectiveOpenAIConfig("  https://example.com/v1  ", "  config-model  ", "  flag-model  ")
	require.NoError(t, err)
	require.Equal(t, "https://example.com/v1", effectiveConfig.BaseURL)
	require.Equal(t, "flag-model", effectiveConfig.Model)
}

func TestResolveEffectiveOpenAIConfigRejectsInvalidFullURL(t *testing.T) {
	_, err := ResolveEffectiveOpenAIConfig("not-a-url", "custom-model", "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "valid http(s) URL")
}
