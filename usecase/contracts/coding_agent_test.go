package contracts

import (
	"context"
	"testing"

	"bentos-backend/domain"
	"github.com/stretchr/testify/require"
)

type dummyCodingAgent struct{}

func (a *dummyCodingAgent) Run(_ context.Context, task string, _ domain.CodingAgentRunOptions) (domain.CodingAgentRunResult, error) {
	return domain.CodingAgentRunResult{Text: task}, nil
}

type dummyCodingAgentEnvironment struct{}

func (e *dummyCodingAgentEnvironment) SetupAgent(_ context.Context, _ domain.CodingAgentSetupOptions) (CodingAgent, error) {
	return &dummyCodingAgent{}, nil
}

func TestCodingAgentEnvironmentContract(t *testing.T) {
	var env CodingAgentEnvironment = &dummyCodingAgentEnvironment{}

	agent, err := env.SetupAgent(context.Background(), domain.CodingAgentSetupOptions{
		RepoURL: "https://github.com/example/repo.git",
	})
	require.NoError(t, err)

	result, err := agent.Run(context.Background(), "Task abc", domain.CodingAgentRunOptions{
		Provider: "openai",
		Model:    "o4-mini",
	})
	require.NoError(t, err)
	require.Equal(t, "Task abc", result.Text)
}
