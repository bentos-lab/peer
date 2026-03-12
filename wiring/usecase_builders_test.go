package wiring

import (
	"context"
	"errors"
	"testing"

	"bentos-backend/config"
	"bentos-backend/domain"
	"bentos-backend/usecase/contracts"
	"github.com/stretchr/testify/require"
)

type fakeCodeEnvironment struct {
	cleanupCalled bool
	cleanupErr    error
}

func (f *fakeCodeEnvironment) SetupAgent(context.Context, domain.CodingAgentSetupOptions) (contracts.CodingAgent, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeCodeEnvironment) LoadChangedFiles(context.Context, domain.CodeEnvironmentLoadOptions) ([]domain.ChangedFile, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeCodeEnvironment) ReadFile(context.Context, string, string) (string, bool, error) {
	return "", false, errors.New("not implemented")
}

func (f *fakeCodeEnvironment) PushChanges(context.Context, domain.CodeEnvironmentPushOptions) (domain.CodeEnvironmentPushResult, error) {
	return domain.CodeEnvironmentPushResult{}, errors.New("not implemented")
}

func (f *fakeCodeEnvironment) Cleanup(context.Context) error {
	f.cleanupCalled = true
	return f.cleanupErr
}

type fakeCodeEnvironmentFactory struct {
	lastRepoURL string
	env         *fakeCodeEnvironment
	err         error
}

func (f *fakeCodeEnvironmentFactory) New(_ context.Context, opts domain.CodeEnvironmentInitOptions) (contracts.CodeEnvironment, error) {
	f.lastRepoURL = opts.RepoURL
	if f.err != nil {
		return nil, f.err
	}
	if f.env == nil {
		f.env = &fakeCodeEnvironment{}
	}
	return f.env, nil
}

func TestPreInitRepoInvokesFactoryAndCleanup(t *testing.T) {
	factory := &fakeCodeEnvironmentFactory{env: &fakeCodeEnvironment{}}
	err := preInitRepo(factory, "https://example.com/repo.git")
	require.NoError(t, err)
	require.Equal(t, "https://example.com/repo.git", factory.lastRepoURL)
	require.True(t, factory.env.cleanupCalled)
}

func TestPreInitRepoFailsWhenFactoryReturnsError(t *testing.T) {
	expectedErr := errors.New("boom")
	factory := &fakeCodeEnvironmentFactory{err: expectedErr}
	err := preInitRepo(factory, "https://example.com/repo.git")
	require.ErrorIs(t, err, expectedErr)
}

func TestBuildChangeRequestUseCaseRejectsMissingCodingAgent(t *testing.T) {
	_, err := BuildChangeRequestUseCase(config.Config{}, CLILLMOptions{ForceCLIPublishers: true}, "", "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "coding agent")
}

func TestBuildAutogenUseCaseRejectsMissingCodingAgent(t *testing.T) {
	_, err := BuildAutogenUseCase(config.Config{}, CLILLMOptions{ForceCLIPublishers: true}, "", "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "coding agent")
}

func TestBuildReplyCommentUseCaseRejectsMissingCodingAgent(t *testing.T) {
	_, err := BuildReplyCommentUseCase(config.Config{}, CLILLMOptions{ForceCLIPublishers: true}, "", "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "coding agent")
}
