package wiring

import (
	"testing"

	"bentos-backend/config"
	"github.com/stretchr/testify/require"
)

func TestResolveCLICodingAgentConfigUsesConfigWhenNoOverrides(t *testing.T) {
	cfg := config.Config{
		CodingAgent: config.CodingAgentConfig{
			Agent:    "opencode",
			Provider: "openai",
			Model:    "gpt-4.1-mini",
		},
	}

	resolved := ResolveCLICodingAgentConfig(cfg, CLILLMOptions{})
	require.Equal(t, cfg.CodingAgent, resolved)
}

func TestResolveCLICodingAgentConfigAppliesOverrides(t *testing.T) {
	cfg := config.Config{
		CodingAgent: config.CodingAgentConfig{
			Agent:    "opencode",
			Provider: "openai",
			Model:    "gpt-4.1-mini",
		},
	}

	resolved := ResolveCLICodingAgentConfig(cfg, CLILLMOptions{
		CodeAgent:            "agent-x",
		CodeAgentSet:         true,
		CodeAgentProvider:    "provider-y",
		CodeAgentProviderSet: true,
		CodeAgentModel:       "model-z",
		CodeAgentModelSet:    true,
	})
	require.Equal(t, "agent-x", resolved.Agent)
	require.Equal(t, "provider-y", resolved.Provider)
	require.Equal(t, "model-z", resolved.Model)
}
