package llm

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveBaseURLAndModelShortcutUsesDefaultWhenFlagModelMissing(t *testing.T) {
	baseURL, model, isShortcut, err := ResolveBaseURLAndModel("gemini", "ignored-config-model", "")
	require.NoError(t, err)
	require.True(t, isShortcut)
	require.Equal(t, "https://generativelanguage.googleapis.com/v1beta/openai", baseURL)
	require.Equal(t, "gemini-2.5-flash-lite", model)
}

func TestResolveBaseURLAndModelShortcutUsesFlagModelWhenProvided(t *testing.T) {
	baseURL, model, isShortcut, err := ResolveBaseURLAndModel("openai", "cfg-model", "gpt-4.1")
	require.NoError(t, err)
	require.True(t, isShortcut)
	require.Equal(t, "https://api.openai.com/v1", baseURL)
	require.Equal(t, "gpt-4.1", model)
}

func TestResolveBaseURLAndModelFullURLRequiresModel(t *testing.T) {
	_, _, isShortcut, err := ResolveBaseURLAndModel("https://example.com/v1", "", "")
	require.Error(t, err)
	require.False(t, isShortcut)
}

func TestResolveBaseURLAndModelFullURLUsesConfigModelFallback(t *testing.T) {
	baseURL, model, isShortcut, err := ResolveBaseURLAndModel("https://example.com/v1", "my-model", "")
	require.NoError(t, err)
	require.False(t, isShortcut)
	require.Equal(t, "https://example.com/v1", baseURL)
	require.Equal(t, "my-model", model)
}

func TestResolveBaseURLAndModelAnthropicShortcut(t *testing.T) {
	baseURL, model, isShortcut, err := ResolveBaseURLAndModel("anthropic", "", "")
	require.NoError(t, err)
	require.True(t, isShortcut)
	require.Equal(t, "https://api.anthropic.com/v1", baseURL)
	require.Equal(t, "claude-3-5-haiku-latest", model)
}
