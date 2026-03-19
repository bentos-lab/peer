package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/bentos-lab/peer/config"
	"github.com/bentos-lab/peer/domain"
	"github.com/bentos-lab/peer/usecase"
	"github.com/stretchr/testify/require"
)

type fakeCommitUseCase struct {
	requests  []usecase.CommitRequest
	result    usecase.CommitExecutionResult
	second    usecase.CommitExecutionResult
	secondErr error
}

func (f *fakeCommitUseCase) Execute(_ context.Context, request usecase.CommitRequest) (usecase.CommitExecutionResult, error) {
	f.requests = append(f.requests, request)
	if len(f.requests) == 1 {
		return f.result, nil
	}
	if f.secondErr != nil {
		return usecase.CommitExecutionResult{}, f.secondErr
	}
	return f.second, nil
}

func TestCommitCommandUsesStagedModeWhenRequested(t *testing.T) {
	useCase := &fakeCommitUseCase{
		result: usecase.CommitExecutionResult{CommitMessage: "feat: staged"},
	}
	cmd := NewCommitCommand(func(_ string) (usecase.CommitUseCase, error) { return useCase, nil }, &testCodeEnvironmentFactory{}, nil)

	var stdout bytes.Buffer
	stdin := strings.NewReader("y\n")
	confirm := true
	err := cmd.Run(context.Background(), config.Config{}, CommitRunParams{Staged: true, Confirm: &confirm}, &stdout, stdin)
	require.NoError(t, err)
	require.Len(t, useCase.requests, 2)
	require.False(t, useCase.requests[0].StageAll)
}

func TestCommitCommandSkipsPromptWhenConfirmTrue(t *testing.T) {
	useCase := &fakeCommitUseCase{
		result: usecase.CommitExecutionResult{CommitMessage: "feat: msg"},
		second: usecase.CommitExecutionResult{CommitMessage: "feat: msg", Committed: true},
	}
	cmd := NewCommitCommand(func(_ string) (usecase.CommitUseCase, error) { return useCase, nil }, &testCodeEnvironmentFactory{}, nil)
	cmd.promptYesNo = func(_ io.Writer, _ io.Reader, _ string) (bool, error) {
		return false, errors.New("unexpected prompt")
	}

	var stdout bytes.Buffer
	stdin := strings.NewReader("")
	confirm := true
	err := cmd.Run(context.Background(), config.Config{}, CommitRunParams{Confirm: &confirm}, &stdout, stdin)
	require.NoError(t, err)
	require.Len(t, useCase.requests, 2)
}

func TestCommitCommandStopsWhenPromptRejected(t *testing.T) {
	useCase := &fakeCommitUseCase{
		result: usecase.CommitExecutionResult{CommitMessage: "feat: msg"},
	}
	cmd := NewCommitCommand(func(_ string) (usecase.CommitUseCase, error) { return useCase, nil }, &testCodeEnvironmentFactory{}, nil)

	var stdout bytes.Buffer
	stdin := strings.NewReader("n\n")
	err := cmd.Run(context.Background(), config.Config{}, CommitRunParams{}, &stdout, stdin)
	require.NoError(t, err)
	require.Len(t, useCase.requests, 1)
}

func TestCommitCommandSkipsCommitWhenConfirmFalse(t *testing.T) {
	useCase := &fakeCommitUseCase{
		result: usecase.CommitExecutionResult{CommitMessage: "feat: msg"},
	}
	cmd := NewCommitCommand(func(_ string) (usecase.CommitUseCase, error) { return useCase, nil }, &testCodeEnvironmentFactory{}, nil)

	var stdout bytes.Buffer
	stdin := strings.NewReader("")
	confirm := false
	err := cmd.Run(context.Background(), config.Config{}, CommitRunParams{Confirm: &confirm}, &stdout, stdin)
	require.NoError(t, err)
	require.Len(t, useCase.requests, 1)
}

func TestCommitCommandReturnsNoChangesError(t *testing.T) {
	useCase := &fakeCommitUseCase{
		result:    usecase.CommitExecutionResult{CommitMessage: "feat: msg"},
		secondErr: domain.ErrNoCodeChanges,
	}
	cmd := NewCommitCommand(func(_ string) (usecase.CommitUseCase, error) { return useCase, nil }, &testCodeEnvironmentFactory{}, nil)

	var stdout bytes.Buffer
	stdin := strings.NewReader("y\n")
	err := cmd.Run(context.Background(), config.Config{}, CommitRunParams{}, &stdout, stdin)
	require.ErrorIs(t, err, domain.ErrNoCodeChanges)
}
