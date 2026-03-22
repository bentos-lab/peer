package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/bentos-lab/peer/domain"
	"github.com/bentos-lab/peer/usecase/contracts"
	"github.com/stretchr/testify/require"
)

type commitTestGenerator struct {
	message     string
	called      bool
	lastPayload CommitMessagePayload
}

func (g *commitTestGenerator) GenerateCommitMessage(_ context.Context, payload CommitMessagePayload) (string, error) {
	g.called = true
	g.lastPayload = payload
	return g.message, nil
}

type commitTestEnv struct {
	lastCommit domain.CodeEnvironmentCommitOptions
	commitErr  error
}

func (e *commitTestEnv) SetupAgent(_ context.Context, _ domain.CodingAgentSetupOptions) (contracts.CodingAgent, error) {
	return nil, nil
}

func (e *commitTestEnv) LoadChangedFiles(_ context.Context, _ domain.CodeEnvironmentLoadOptions) ([]domain.ChangedFile, error) {
	return nil, nil
}

func (e *commitTestEnv) EnsureDiffContentAvailable(_ context.Context, _ domain.CodeEnvironmentLoadOptions) error {
	return nil
}

func (e *commitTestEnv) ReadFile(_ context.Context, _ string, _ string) (string, bool, error) {
	return "", false, nil
}

func (e *commitTestEnv) CommitChanges(_ context.Context, opts domain.CodeEnvironmentCommitOptions) (domain.CodeEnvironmentCommitResult, error) {
	e.lastCommit = opts
	if e.commitErr != nil {
		return domain.CodeEnvironmentCommitResult{}, e.commitErr
	}
	return domain.CodeEnvironmentCommitResult{Committed: true}, nil
}

func (e *commitTestEnv) PushChanges(_ context.Context, _ domain.CodeEnvironmentPushOptions) (domain.CodeEnvironmentPushResult, error) {
	return domain.CodeEnvironmentPushResult{}, nil
}

func (e *commitTestEnv) Cleanup(_ context.Context) error {
	return nil
}

func TestCommitUseCaseGeneratesMessageWhenMissing(t *testing.T) {
	generator := &commitTestGenerator{message: "feat: test"}
	useCase, err := NewCommitUseCase(generator, nil)
	require.NoError(t, err)

	env := &commitTestEnv{}
	result, err := useCase.Execute(context.Background(), CommitRequest{
		Commit:      false,
		Environment: env,
	})
	require.NoError(t, err)
	require.True(t, generator.called)
	require.Equal(t, "feat: test", result.CommitMessage)
	require.False(t, result.Committed)
	require.True(t, generator.lastPayload.Staged)
}

func TestCommitUseCaseCommitsWithProvidedMessage(t *testing.T) {
	generator := &commitTestGenerator{message: "feat: unused"}
	useCase, err := NewCommitUseCase(generator, nil)
	require.NoError(t, err)

	env := &commitTestEnv{}
	result, err := useCase.Execute(context.Background(), CommitRequest{
		CommitMessage: "fix: commit",
		Commit:        true,
		StageAll:      true,
		Environment:   env,
	})
	require.NoError(t, err)
	require.Equal(t, "fix: commit", result.CommitMessage)
	require.True(t, result.Committed)
	require.Equal(t, "fix: commit", env.lastCommit.CommitMessage)
	require.True(t, env.lastCommit.StageAll)
	require.False(t, generator.called)
}

func TestCommitUseCaseRejectsEmptyGeneratedMessage(t *testing.T) {
	generator := &commitTestGenerator{message: "  "}
	useCase, err := NewCommitUseCase(generator, nil)
	require.NoError(t, err)

	env := &commitTestEnv{}
	_, err = useCase.Execute(context.Background(), CommitRequest{
		Commit:      false,
		Environment: env,
	})
	require.Error(t, err)
}

func TestCommitUseCaseReturnsCommitError(t *testing.T) {
	generator := &commitTestGenerator{message: "feat: msg"}
	useCase, err := NewCommitUseCase(generator, nil)
	require.NoError(t, err)

	env := &commitTestEnv{commitErr: domain.ErrNoCodeChanges}
	_, err = useCase.Execute(context.Background(), CommitRequest{
		Commit:      true,
		StageAll:    false,
		Environment: env,
	})
	require.ErrorIs(t, err, domain.ErrNoCodeChanges)
	require.True(t, errors.Is(err, domain.ErrNoCodeChanges))
}
