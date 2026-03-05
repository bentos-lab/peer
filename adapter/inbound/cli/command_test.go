package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	cliinput "bentos-backend/adapter/outbound/input/cli"
	"bentos-backend/domain"
	"bentos-backend/usecase"
	"github.com/stretchr/testify/require"
)

type fakeReviewUseCase struct {
	lastRequest usecase.ReviewRequest
	executed    bool
	err         error
}

func (f *fakeReviewUseCase) Execute(_ context.Context, request usecase.ReviewRequest) (usecase.ReviewExecutionResult, error) {
	f.executed = true
	f.lastRequest = request
	return usecase.ReviewExecutionResult{}, f.err
}

type spyLogger struct {
	events []string
}

func (s *spyLogger) Infof(format string, args ...any) {
	s.events = append(s.events, "info:"+fmt.Sprintf(format, args...))
}

func (s *spyLogger) Debugf(format string, args ...any) {
	s.events = append(s.events, "debug:"+fmt.Sprintf(format, args...))
}

func (s *spyLogger) Errorf(format string, args ...any) {
	s.events = append(s.events, "error:"+fmt.Sprintf(format, args...))
}

func TestCommand_RunMapsParamsToMetadata(t *testing.T) {
	fakeUC := &fakeReviewUseCase{}
	logger := &spyLogger{}
	command := NewLocalCommand(fakeUC, logger)

	err := command.Run(context.Background(), RunParams{
		IncludeUnstaged:  true,
		IncludeUntracked: true,
	})
	require.NoError(t, err)
	require.True(t, fakeUC.executed)
	require.Equal(t, "local/repo", fakeUC.lastRequest.Repository)
	require.Equal(t, "true", fakeUC.lastRequest.Metadata[cliinput.MetadataKeyAutoIncludeAll])
	require.Equal(t, "true", fakeUC.lastRequest.Metadata[cliinput.MetadataKeyAutoIncludeUntracked])
	require.Equal(t, "", fakeUC.lastRequest.Metadata[cliinput.MetadataKeyChangedFiles])
	require.True(t, containsEvent(logger.events, "info:CLI review started."))
	require.True(t, containsEvent(logger.events, "info:CLI review completed."))
}

func TestCommand_RunKeepsChangedFilesOverride(t *testing.T) {
	fakeUC := &fakeReviewUseCase{}
	command := NewLocalCommand(fakeUC, nil)

	err := command.Run(context.Background(), RunParams{
		ChangedFiles:    "a.go,b.go",
		IncludeUnstaged: true,
	})
	require.NoError(t, err)
	require.Equal(t, "a.go,b.go", fakeUC.lastRequest.Metadata[cliinput.MetadataKeyChangedFiles])
}

func TestCommand_RunGitHubPRMapsRequest(t *testing.T) {
	fakeUC := &fakeReviewUseCase{}
	command := NewGitHubPRCommand(fakeUC, nil)

	err := command.Run(context.Background(), RunParams{
		PRNumber: 7,
	})
	require.NoError(t, err)
	require.True(t, fakeUC.executed)
	require.Equal(t, 7, fakeUC.lastRequest.ChangeRequestNumber)
	require.Equal(t, "", fakeUC.lastRequest.Repository)
	require.Empty(t, fakeUC.lastRequest.Metadata)
}

func TestCommand_RunReturnsErrorWhenProviderIsNotConfigured(t *testing.T) {
	fakeUC := &fakeReviewUseCase{}
	command := &Command{
		reviewer:     fakeUC,
		providerName: domain.ReviewInputProvider("unknown"),
		logger:       usecase.NopLogger,
	}

	err := command.Run(context.Background(), RunParams{})
	require.Error(t, err)
	require.False(t, fakeUC.executed)
}

func TestCommand_RunReturnsErrorWhenReviewerIsNotConfigured(t *testing.T) {
	command := NewLocalCommand(nil, nil)

	err := command.Run(context.Background(), RunParams{})
	require.Error(t, err)
}

func TestCommand_RunLogsFailure(t *testing.T) {
	fakeUC := &fakeReviewUseCase{err: errors.New("boom")}
	logger := &spyLogger{}
	command := NewLocalCommand(fakeUC, logger)

	err := command.Run(context.Background(), RunParams{})
	require.Error(t, err)
	require.True(t, containsEvent(logger.events, "error:CLI review failed."))
}

func containsEvent(events []string, target string) bool {
	for _, event := range events {
		if strings.Contains(event, target) {
			return true
		}
	}
	return false
}
