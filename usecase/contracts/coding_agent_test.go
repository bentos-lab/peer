package contracts

import (
	"context"
	"testing"

	"github.com/bentos-lab/peer/domain"
	"github.com/stretchr/testify/require"
)

type dummyCodingAgent struct{}

func (a *dummyCodingAgent) Run(_ context.Context, task string, _ domain.CodingAgentRunOptions) (domain.CodingAgentRunResult, error) {
	return domain.CodingAgentRunResult{Text: task}, nil
}

type dummyCodeEnvironment struct{}

func (e *dummyCodeEnvironment) SetupAgent(_ context.Context, _ domain.CodingAgentSetupOptions) (CodingAgent, error) {
	return &dummyCodingAgent{}, nil
}

func (e *dummyCodeEnvironment) LoadChangedFiles(_ context.Context, _ domain.CodeEnvironmentLoadOptions) ([]domain.ChangedFile, error) {
	return []domain.ChangedFile{}, nil
}

func (e *dummyCodeEnvironment) ReadFile(_ context.Context, _ string, _ string) (string, bool, error) {
	return "", false, nil
}

func (e *dummyCodeEnvironment) CommitChanges(_ context.Context, _ domain.CodeEnvironmentCommitOptions) (domain.CodeEnvironmentCommitResult, error) {
	return domain.CodeEnvironmentCommitResult{}, nil
}

func (e *dummyCodeEnvironment) PushChanges(_ context.Context, _ domain.CodeEnvironmentPushOptions) (domain.CodeEnvironmentPushResult, error) {
	return domain.CodeEnvironmentPushResult{}, nil
}

func (e *dummyCodeEnvironment) Cleanup(_ context.Context) error {
	return nil
}

type dummyCodeEnvironmentFactory struct{}

func (f *dummyCodeEnvironmentFactory) New(_ context.Context, _ domain.CodeEnvironmentInitOptions) (CodeEnvironment, error) {
	return &dummyCodeEnvironment{}, nil
}

func TestCodeEnvironmentContract(t *testing.T) {
	var env CodeEnvironment = &dummyCodeEnvironment{}

	agent, err := env.SetupAgent(context.Background(), domain.CodingAgentSetupOptions{
		Agent: "opencode",
		Ref:   "main",
	})
	require.NoError(t, err)

	result, err := agent.Run(context.Background(), "Task abc", domain.CodingAgentRunOptions{
		Provider: "openai",
		Model:    "o4-mini",
	})
	require.NoError(t, err)
	require.Equal(t, "Task abc", result.Text)
}

func TestCodeEnvironmentFactoryContract(t *testing.T) {
	var factory CodeEnvironmentFactory = &dummyCodeEnvironmentFactory{}

	env, err := factory.New(context.Background(), domain.CodeEnvironmentInitOptions{
		RepoURL: "https://github.com/example/repo.git",
	})
	require.NoError(t, err)
	require.NotNil(t, env)
}
