package cli

import (
	"context"
	"testing"

	"github.com/bentos-lab/peer/config"
	"github.com/bentos-lab/peer/usecase"

	"github.com/stretchr/testify/require"
)

type fakeOverviewUseCase struct {
	requests []usecase.OverviewRequest
}

func (f *fakeOverviewUseCase) Execute(_ context.Context, request usecase.OverviewRequest) (usecase.OverviewExecutionResult, error) {
	f.requests = append(f.requests, request)
	return usecase.OverviewExecutionResult{}, nil
}

func TestOverviewCommandSkipsResolveRepositoryWhenChangeRequestEmptyWithRepo(t *testing.T) {
	overviewUC := &fakeOverviewUseCase{}
	githubClient := &fakeGitHubClient{resolvedRepository: "owner/repo"}
	builder := func(_ string) (usecase.OverviewUseCase, error) {
		return overviewUC, nil
	}
	resolver := StaticVCSClients{GitHub: githubClient}
	command := NewOverviewCommand(builder, resolver, &testCodeEnvironmentFactory{}, &testRecipeLoader{}, nil)

	err := command.Run(context.Background(), config.Config{}, OverviewParams{
		VCSProvider: "github",
		Repo:        "owner/repo",
	})

	require.NoError(t, err)
	require.Equal(t, 0, githubClient.resolveRepositoryCalls)
	require.Len(t, overviewUC.requests, 1)
	require.Equal(t, "owner/repo", overviewUC.requests[0].Input.Target.Repository)
	require.NotEmpty(t, overviewUC.requests[0].Input.RepoURL)
}

func TestOverviewCommandSkipsResolveRepositoryWhenChangeRequestEmptyNoRepo(t *testing.T) {
	overviewUC := &fakeOverviewUseCase{}
	githubClient := &fakeGitHubClient{resolvedRepository: "owner/repo"}
	builder := func(_ string) (usecase.OverviewUseCase, error) {
		return overviewUC, nil
	}
	resolver := StaticVCSClients{GitHub: githubClient}
	command := NewOverviewCommand(builder, resolver, &testCodeEnvironmentFactory{}, &testRecipeLoader{}, nil)

	err := command.Run(context.Background(), config.Config{}, OverviewParams{
		VCSProvider: "github",
	})

	require.NoError(t, err)
	require.Equal(t, 0, githubClient.resolveRepositoryCalls)
	require.Len(t, overviewUC.requests, 1)
	require.Equal(t, "local", overviewUC.requests[0].Input.Target.Repository)
	require.Empty(t, overviewUC.requests[0].Input.RepoURL)
}
