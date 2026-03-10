package wiring

import (
	"testing"

	"bentos-backend/config"
	"github.com/stretchr/testify/require"
)

func TestBuildCLIReviewCommandBuildsWithCodingAgentWiring(t *testing.T) {
	command, err := BuildCLIReviewCommand(config.Config{
		LogLevel: "info",
		OpenAI: config.OpenAIConfig{
			BaseURL: "openai",
			APIKey:  "key",
			Model:   "gpt-4.1-mini",
		},
		CodingAgent: config.CodingAgentConfig{
			Agent:    "opencode",
			Provider: "openai",
			Model:    "gpt-4.1-mini",
		},
	}, CLILLMOptions{}, "")

	require.NoError(t, err)
	require.NotNil(t, command)
}

func TestBuildCLIReviewCommandRejectsMissingOpenAIAPIKey(t *testing.T) {
	_, err := BuildCLIReviewCommand(config.Config{
		LogLevel: "info",
		OpenAI: config.OpenAIConfig{
			BaseURL: "openai",
			Model:   "gpt-4.1-mini",
		},
		CodingAgent: config.CodingAgentConfig{
			Agent:    "opencode",
			Provider: "openai",
			Model:    "gpt-4.1-mini",
		},
	}, CLILLMOptions{}, "")

	require.Error(t, err)
	require.Contains(t, err.Error(), "openai API key is required")
}

func TestBuildCLIReviewCommandRejectsMissingCodingAgentModel(t *testing.T) {
	_, err := BuildCLIReviewCommand(config.Config{
		LogLevel: "info",
		OpenAI: config.OpenAIConfig{
			BaseURL: "openai",
			APIKey:  "key",
			Model:   "",
		},
		CodingAgent: config.CodingAgentConfig{
			Agent:    "opencode",
			Provider: "openai",
			Model:    "",
		},
	}, CLILLMOptions{}, "")

	require.Error(t, err)
	require.Contains(t, err.Error(), "coding agent model is required")
}
