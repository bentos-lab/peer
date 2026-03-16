package cli

import (
	"context"
	"testing"

	"bentos-backend/config"
	"bentos-backend/domain"
	"bentos-backend/usecase"

	"github.com/stretchr/testify/require"
)

type fakeAutogenUseCase struct {
	requests []usecase.AutogenRequest
}

func (f *fakeAutogenUseCase) Execute(_ context.Context, request usecase.AutogenRequest) (usecase.AutogenExecutionResult, error) {
	f.requests = append(f.requests, request)
	return usecase.AutogenExecutionResult{}, nil
}

func TestAutogenCommandRejectsChangeRequestWithBaseHead(t *testing.T) {
	useCase := &fakeAutogenUseCase{}
	githubClient := &fakeGitHubClient{}
	builder := func(_ string) (usecase.AutogenUseCase, error) {
		return useCase, nil
	}
	resolver := StaticVCSClients{GitHub: githubClient}
	cmd := NewAutogenCommand(builder, resolver, &testCodeEnvironmentFactory{}, &testRecipeLoader{}, nil)

	err := cmd.Run(context.Background(), config.Config{}, AutogenRunParams{
		VCSProvider:   "github",
		Repo:          "org/repo",
		ChangeRequest: "123",
		Base:          "main",
		Head:          "feature",
		Docs:          new(true),
	})
	require.ErrorContains(t, err, "--change-request")
}

func TestAutogenCommandRequiresChangeRequestForPublish(t *testing.T) {
	useCase := &fakeAutogenUseCase{}
	githubClient := &fakeGitHubClient{}
	builder := func(_ string) (usecase.AutogenUseCase, error) {
		return useCase, nil
	}
	resolver := StaticVCSClients{GitHub: githubClient}
	cmd := NewAutogenCommand(builder, resolver, &testCodeEnvironmentFactory{}, &testRecipeLoader{}, nil)

	err := cmd.Run(context.Background(), config.Config{}, AutogenRunParams{
		VCSProvider: "github",
		Publish:     true,
		Docs:        new(true),
	})
	require.ErrorContains(t, err, "--publish")
}

func TestAutogenCommandDefaultsLocalWorkspace(t *testing.T) {
	useCase := &fakeAutogenUseCase{}
	githubClient := &fakeGitHubClient{resolvedRepository: "org/repo"}
	builder := func(_ string) (usecase.AutogenUseCase, error) {
		return useCase, nil
	}
	resolver := StaticVCSClients{GitHub: githubClient}
	cmd := NewAutogenCommand(builder, resolver, &testCodeEnvironmentFactory{}, &testRecipeLoader{}, nil)

	err := cmd.Run(context.Background(), config.Config{}, AutogenRunParams{
		VCSProvider: "github",
		Docs:        new(true),
	})
	require.NoError(t, err)
	require.Len(t, useCase.requests, 1)
	require.Equal(t, "@all", useCase.requests[0].Input.Head)
	require.Equal(t, "HEAD", useCase.requests[0].Input.Base)
}

func TestAutogenCommandRespectsBaseWhenHeadEmpty(t *testing.T) {
	useCase := &fakeAutogenUseCase{}
	githubClient := &fakeGitHubClient{resolvedRepository: "org/repo"}
	builder := func(_ string) (usecase.AutogenUseCase, error) {
		return useCase, nil
	}
	resolver := StaticVCSClients{GitHub: githubClient}
	cmd := NewAutogenCommand(builder, resolver, &testCodeEnvironmentFactory{}, &testRecipeLoader{}, nil)

	err := cmd.Run(context.Background(), config.Config{}, AutogenRunParams{
		VCSProvider: "github",
		Base:        "main",
		Head:        "",
		Docs:        new(true),
	})
	require.NoError(t, err)
	require.Len(t, useCase.requests, 1)
	require.Equal(t, "main", useCase.requests[0].Input.Base)
	require.Equal(t, "@all", useCase.requests[0].Input.Head)
}

func TestAutogenCommandUsesPullRequestInfo(t *testing.T) {
	useCase := &fakeAutogenUseCase{}
	githubClient := &fakeGitHubClient{
		resolvedRepository: "org/repo",
		pullRequestInfo: domain.ChangeRequestInfo{
			Repository:  "org/repo",
			Number:      7,
			Title:       "title",
			Description: "desc",
			BaseRef:     "baseSHA",
			HeadRef:     "headSHA",
			HeadRefName: "feature-branch",
		},
	}
	builder := func(_ string) (usecase.AutogenUseCase, error) {
		return useCase, nil
	}
	resolver := StaticVCSClients{GitHub: githubClient}
	cmd := NewAutogenCommand(builder, resolver, &testCodeEnvironmentFactory{}, &testRecipeLoader{}, nil)

	err := cmd.Run(context.Background(), config.Config{}, AutogenRunParams{
		VCSProvider:   "github",
		ChangeRequest: "7",
		Publish:       true,
		Docs:          new(true),
		Tests:         new(true),
	})
	require.NoError(t, err)
	require.Len(t, useCase.requests, 1)
	require.Equal(t, "baseSHA", useCase.requests[0].Input.Base)
	require.Equal(t, "headSHA", useCase.requests[0].Input.Head)
	require.Equal(t, "feature-branch", useCase.requests[0].HeadBranch)
	require.True(t, useCase.requests[0].Publish)
}
