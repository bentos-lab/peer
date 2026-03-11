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
			BaseURL: "",
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

func TestBuildCLIReviewCommandAllowsMissingCodingAgentModel(t *testing.T) {
	command, err := BuildCLIReviewCommand(config.Config{
		LogLevel: "info",
		OpenAI: config.OpenAIConfig{
			BaseURL: "",
		},
		CodingAgent: config.CodingAgentConfig{
			Agent:    "opencode",
			Provider: "openai",
			Model:    "",
		},
	}, CLILLMOptions{}, "")
	require.NoError(t, err)
	require.NotNil(t, command)
}
