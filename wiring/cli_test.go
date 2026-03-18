package wiring

import (
	"testing"

	"bentos-backend/config"
	"github.com/stretchr/testify/require"
)

func TestResolveCLILLMConfigUsesShortcutDefaultModelWhenFlagMissing(t *testing.T) {
	cfg := config.Config{
		OpenAIBaseURL: "gemini",
		OpenAIModel:   "env-model",
		OpenAIAPIKey:  "env-key",
	}
	llmCfg, err := resolveCLILLMConfig(cfg, CLILLMOptions{
		OpenAIBaseURL: "openai",
	})
	require.NoError(t, err)
	require.Equal(t, "https://api.openai.com/v1", llmCfg.BaseURL)
	require.Equal(t, "gpt-4.1-mini", llmCfg.Model)
	require.Equal(t, "env-key", llmCfg.APIKey)
}

func TestResolveCLILLMConfigFullURLRequiresResolvedModel(t *testing.T) {
	cfg := config.Config{
		OpenAIBaseURL: "gemini",
		OpenAIModel:   "",
		OpenAIAPIKey:  "env-key",
	}
	_, err := resolveCLILLMConfig(cfg, CLILLMOptions{
		OpenAIBaseURL: "https://example.com/v1",
	})
	require.Error(t, err)
}

func TestResolveCLILLMConfigUsesFlagOverrides(t *testing.T) {
	cfg := config.Config{
		OpenAIBaseURL: "gemini",
		OpenAIModel:   "env-model",
		OpenAIAPIKey:  "env-key",
	}
	llmCfg, err := resolveCLILLMConfig(cfg, CLILLMOptions{
		OpenAIBaseURL: "https://example.com/v1",
		OpenAIModel:   "flag-model",
		OpenAIAPIKey:  "flag-key",
	})
	require.NoError(t, err)
	require.Equal(t, "https://example.com/v1", llmCfg.BaseURL)
	require.Equal(t, "flag-model", llmCfg.Model)
	require.Equal(t, "flag-key", llmCfg.APIKey)
}

func TestResolveCLILLMConfigRequiresAPIKey(t *testing.T) {
	cfg := config.Config{
		OpenAIBaseURL: "gemini",
		OpenAIModel:   "env-model",
		OpenAIAPIKey:  "",
	}
	_, err := resolveCLILLMConfig(cfg, CLILLMOptions{})
	require.Error(t, err)
}
