package codingagent

import (
	"context"
	"testing"

	"github.com/bentos-lab/peer/domain"
	"github.com/bentos-lab/peer/usecase"
	"github.com/bentos-lab/peer/usecase/contracts"
	"github.com/stretchr/testify/require"
)

type commitTestAgent struct {
	lastTask string
	result   string
}

func (a *commitTestAgent) Run(_ context.Context, task string, _ domain.CodingAgentRunOptions) (domain.CodingAgentRunResult, error) {
	a.lastTask = task
	return domain.CodingAgentRunResult{Text: a.result}, nil
}

type commitTestEnvironment struct {
	agent        *commitTestAgent
	changedFiles []domain.ChangedFile
}

func (e *commitTestEnvironment) SetupAgent(_ context.Context, _ domain.CodingAgentSetupOptions) (contracts.CodingAgent, error) {
	return e.agent, nil
}

func (e *commitTestEnvironment) ResolveBaseHead(_ context.Context, base string, head string) (string, string, error) {
	return base, head, nil
}

func (e *commitTestEnvironment) LoadChangedFiles(_ context.Context, _ domain.CodeEnvironmentLoadOptions) ([]domain.ChangedFile, error) {
	return e.changedFiles, nil
}

func (e *commitTestEnvironment) EnsureDiffContentAvailable(_ context.Context, _ domain.CodeEnvironmentLoadOptions) error {
	return nil
}

func (e *commitTestEnvironment) ReadFile(_ context.Context, _ string, _ string) (string, bool, error) {
	return "", false, nil
}

func (e *commitTestEnvironment) CommitChanges(_ context.Context, _ domain.CodeEnvironmentCommitOptions) (domain.CodeEnvironmentCommitResult, error) {
	return domain.CodeEnvironmentCommitResult{}, nil
}

func (e *commitTestEnvironment) PushChanges(_ context.Context, _ domain.CodeEnvironmentPushOptions) (domain.CodeEnvironmentPushResult, error) {
	return domain.CodeEnvironmentPushResult{}, nil
}

func (e *commitTestEnvironment) Cleanup(_ context.Context) error {
	return nil
}

func TestGenerateCommitMessageUsesTaskPromptAndTrimsOutput(t *testing.T) {
	env := &commitTestEnvironment{
		agent:        &commitTestAgent{result: "  feat: add commit  \n"},
		changedFiles: []domain.ChangedFile{{Path: "README.md", DiffSnippet: "diff"}},
	}
	generator, err := NewGenerator(Config{Agent: "opencode", Provider: "openai", Model: "m"}, nil)
	require.NoError(t, err)

	result, err := generator.GenerateCommitMessage(context.Background(), usecase.CommitMessagePayload{
		Staged:      false,
		Environment: env,
	})
	require.NoError(t, err)
	require.Equal(t, "feat: add commit", result)
	require.Contains(t, env.agent.lastTask, "git diff --name-status")
	require.Contains(t, env.agent.lastTask, "Output MUST be a conventional commit message.")
}

func TestGenerateCommitMessageHandlesStagedMode(t *testing.T) {
	env := &commitTestEnvironment{
		agent:        &commitTestAgent{result: "fix: staged"},
		changedFiles: []domain.ChangedFile{{Path: "main.go", DiffSnippet: "diff"}},
	}
	generator, err := NewGenerator(Config{Agent: "opencode", Provider: "openai", Model: "m"}, nil)
	require.NoError(t, err)

	_, err = generator.GenerateCommitMessage(context.Background(), usecase.CommitMessagePayload{
		Staged:      true,
		Environment: env,
	})
	require.NoError(t, err)
	require.Contains(t, env.agent.lastTask, "git diff --cached --name-status")
}
