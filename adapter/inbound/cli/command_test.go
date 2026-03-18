package cli

import (
	"context"
	"testing"

	cliinput "bentos-backend/adapter/outbound/input/cli"
	"bentos-backend/usecase"
	"github.com/stretchr/testify/require"
)

type fakeReviewUseCase struct {
	lastRequest usecase.ReviewRequest
	executed    bool
}

func (f *fakeReviewUseCase) Execute(_ context.Context, request usecase.ReviewRequest) (usecase.ReviewExecutionResult, error) {
	f.executed = true
	f.lastRequest = request
	return usecase.ReviewExecutionResult{}, nil
}

func TestCommand_RunMapsParamsToMetadata(t *testing.T) {
	fakeUC := &fakeReviewUseCase{}
	command := NewCommand(fakeUC)

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
}

func TestCommand_RunKeepsChangedFilesOverride(t *testing.T) {
	fakeUC := &fakeReviewUseCase{}
	command := NewCommand(fakeUC)

	err := command.Run(context.Background(), RunParams{
		ChangedFiles:    "a.go,b.go",
		IncludeUnstaged: true,
	})
	require.NoError(t, err)
	require.Equal(t, "a.go,b.go", fakeUC.lastRequest.Metadata[cliinput.MetadataKeyChangedFiles])
}
