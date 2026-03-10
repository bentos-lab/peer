package cli

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	githubvcs "bentos-backend/adapter/outbound/vcs/github"
	"bentos-backend/usecase"
	"github.com/stretchr/testify/require"
)

func TestCommandRunWithoutRepoKeepsRepoURLEmpty(t *testing.T) {
	changeRequestUC := &fakeChangeRequestUseCase{}
	githubClient := &fakeGitHubClient{
		resolvedRepository: "org/current",
	}
	command := NewCommand(changeRequestUC, githubClient, nil)

	err := command.Run(context.Background(), RunParams{})
	require.NoError(t, err)
	require.Len(t, changeRequestUC.requests, 1)
	require.Equal(t, "org/current", changeRequestUC.requests[0].Repository)
	require.Equal(t, "", changeRequestUC.requests[0].RepoURL)
	require.False(t, changeRequestUC.requests[0].EnableSuggestions)
	require.Equal(t, []string{""}, githubClient.resolveRepositoryInputs)
	require.Empty(t, githubClient.pullRequestInfoInputs)
}

func TestCommandRunWithoutRepoAndWithChangeRequestKeepsRepoURLEmpty(t *testing.T) {
	changeRequestUC := &fakeChangeRequestUseCase{}
	githubClient := &fakeGitHubClient{
		resolvedRepository: "org/current",
		pullRequestInfo: githubvcs.PullRequestInfo{
			Repository:  "org/from-pr",
			BaseRef:     "main",
			HeadRef:     "feature/ref",
			Title:       "title",
			Description: "description",
		},
	}
	command := NewCommand(changeRequestUC, githubClient, nil)

	err := command.Run(context.Background(), RunParams{
		ChangeRequest: "7",
		Comment:       true,
	})
	require.NoError(t, err)
	require.Len(t, changeRequestUC.requests, 1)
	require.Equal(t, "org/from-pr", changeRequestUC.requests[0].Repository)
	require.Equal(t, "", changeRequestUC.requests[0].RepoURL)
	require.Equal(t, []string{""}, githubClient.resolveRepositoryInputs)
	require.Equal(t, []pullRequestInfoInput{{repository: "org/current", number: 7}}, githubClient.pullRequestInfoInputs)
}

func TestCommandRunWithRepoSlugSetsNormalizedRepoURL(t *testing.T) {
	changeRequestUC := &fakeChangeRequestUseCase{}
	githubClient := &fakeGitHubClient{
		resolvedRepository: "owner/repo",
	}
	command := NewCommand(changeRequestUC, githubClient, nil)

	err := command.Run(context.Background(), RunParams{
		Repo: "owner/repo",
	})
	require.NoError(t, err)
	require.Len(t, changeRequestUC.requests, 1)
	require.Equal(t, "owner/repo", changeRequestUC.requests[0].Repository)
	require.Equal(t, "https://github.com/owner/repo.git", changeRequestUC.requests[0].RepoURL)
	require.Equal(t, []string{"owner/repo"}, githubClient.resolveRepositoryInputs)
	require.Empty(t, githubClient.pullRequestInfoInputs)
}

func TestCommandRunWithRepoURLAndChangeRequestUsesPRRepoURL(t *testing.T) {
	changeRequestUC := &fakeChangeRequestUseCase{}
	githubClient := &fakeGitHubClient{
		resolvedRepository: "owner/repo",
		pullRequestInfo: githubvcs.PullRequestInfo{
			Repository:  "org/from-pr",
			BaseRef:     "main",
			HeadRef:     "feature/ref",
			Title:       "title",
			Description: "description",
		},
	}
	command := NewCommand(changeRequestUC, githubClient, nil)

	err := command.Run(context.Background(), RunParams{
		Repo:          "https://github.com/owner/repo.git",
		ChangeRequest: "9",
		Comment:       true,
	})
	require.NoError(t, err)
	require.Len(t, changeRequestUC.requests, 1)
	require.Equal(t, "org/from-pr", changeRequestUC.requests[0].Repository)
	require.Equal(t, "https://github.com/org/from-pr.git", changeRequestUC.requests[0].RepoURL)
	require.Equal(t, []string{"owner/repo"}, githubClient.resolveRepositoryInputs)
	require.Equal(t, []pullRequestInfoInput{{repository: "owner/repo", number: 9}}, githubClient.pullRequestInfoInputs)
}

func TestCommandRunWithSSHRepoURLAndChangeRequestKeepsSSHFormatInPRRepoURL(t *testing.T) {
	changeRequestUC := &fakeChangeRequestUseCase{}
	githubClient := &fakeGitHubClient{
		resolvedRepository: "owner/repo",
		pullRequestInfo: githubvcs.PullRequestInfo{
			Repository:  "org/from-pr",
			BaseRef:     "main",
			HeadRef:     "feature/ref",
			Title:       "title",
			Description: "description",
		},
	}
	command := NewCommand(changeRequestUC, githubClient, nil)

	err := command.Run(context.Background(), RunParams{
		Repo:          "ssh://git@github.com/owner/repo.git",
		ChangeRequest: "9",
		Comment:       true,
	})
	require.NoError(t, err)
	require.Len(t, changeRequestUC.requests, 1)
	require.Equal(t, "org/from-pr", changeRequestUC.requests[0].Repository)
	require.Equal(t, "ssh://git@github.com/org/from-pr.git", changeRequestUC.requests[0].RepoURL)
	require.Equal(t, []string{"owner/repo"}, githubClient.resolveRepositoryInputs)
	require.Equal(t, []pullRequestInfoInput{{repository: "owner/repo", number: 9}}, githubClient.pullRequestInfoInputs)
}

func TestCommandRunWithRepoAndWorkspaceHeadTokenReturnsError(t *testing.T) {
	changeRequestUC := &fakeChangeRequestUseCase{}
	githubClient := &fakeGitHubClient{
		resolvedRepository: "owner/repo",
	}
	command := NewCommand(changeRequestUC, githubClient, nil)

	err := command.Run(context.Background(), RunParams{
		Repo: "owner/repo",
		Head: "@staged",
	})
	require.EqualError(t, err, "--head @staged requires local workspace mode; omit --repo")
	require.Empty(t, changeRequestUC.requests)
	require.Equal(t, []string{"owner/repo"}, githubClient.resolveRepositoryInputs)
}

func TestCommandRunWithoutRepoAndWorkspaceHeadTokenIsAllowed(t *testing.T) {
	changeRequestUC := &fakeChangeRequestUseCase{}
	githubClient := &fakeGitHubClient{
		resolvedRepository: "org/current",
	}
	command := NewCommand(changeRequestUC, githubClient, nil)

	err := command.Run(context.Background(), RunParams{
		Head: "@all",
	})
	require.NoError(t, err)
	require.Len(t, changeRequestUC.requests, 1)
	require.Equal(t, "@all", changeRequestUC.requests[0].Head)
	require.Equal(t, "HEAD", changeRequestUC.requests[0].Base)
	require.Equal(t, "", changeRequestUC.requests[0].RepoURL)
}

func TestCommandRunMapsSuggestFlagToUseCaseRequest(t *testing.T) {
	changeRequestUC := &fakeChangeRequestUseCase{}
	githubClient := &fakeGitHubClient{
		resolvedRepository: "owner/repo",
	}
	command := NewCommand(changeRequestUC, githubClient, nil)

	err := command.Run(context.Background(), RunParams{
		Repo:    "owner/repo",
		Suggest: true,
	})
	require.NoError(t, err)
	require.Len(t, changeRequestUC.requests, 1)
	require.True(t, changeRequestUC.requests[0].EnableSuggestions)
}

func TestCommandRunLogsPreUsecaseSnapshotWithoutSensitiveData(t *testing.T) {
	changeRequestUC := &fakeChangeRequestUseCase{}
	githubClient := &fakeGitHubClient{
		resolvedRepository: "owner/repo",
		pullRequestInfo: githubvcs.PullRequestInfo{
			Repository:  "owner/repo",
			BaseRef:     "main",
			HeadRef:     "feature/ref",
			Title:       "Super secret PR title",
			Description: "Confidential PR description",
		},
	}
	logger := &commandTestSpyLogger{}
	command := NewCommand(changeRequestUC, githubClient, logger)

	err := command.Run(context.Background(), RunParams{
		Repo:          "https://github.com/owner/repo.git",
		ChangeRequest: "9",
		Comment:       true,
		Overview:      true,
		Suggest:       true,
	})

	require.NoError(t, err)
	require.Eventually(t, func() bool {
		return containsLogEvent(logger.events, "info:Pre-usecase input snapshot source=\"cli\" repository=\"owner/repo\" changeRequestNumber=9 enableOverview=true enableSuggestions=true.")
	}, time.Second, 10*time.Millisecond)
	require.Eventually(t, func() bool {
		return containsLogEvent(logger.events, "debug:Pre-usecase input details source=\"cli\" action=\"\" base=\"main\" head=\"feature/ref\"")
	}, time.Second, 10*time.Millisecond)
	require.Eventually(t, func() bool {
		return containsLogEvent(logger.events, "repoURLPresent=true repoURLSafe=\"https://github.com/owner/repo.git\"")
	}, time.Second, 10*time.Millisecond)
	require.Eventually(t, func() bool {
		return containsLogEvent(logger.events, "titleLength=21 descriptionLength=27")
	}, time.Second, 10*time.Millisecond)
	require.False(t, containsLogEvent(logger.events, "Super secret PR title"))
	require.False(t, containsLogEvent(logger.events, "Confidential PR description"))
}

func TestNormalizeRepoAcceptsCommonGitHubFormats(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		input          string
		wantRepository string
		wantRepoURL    string
	}{
		{
			name:           "owner repo slug",
			input:          "sisu-network/bentos-backend",
			wantRepository: "sisu-network/bentos-backend",
			wantRepoURL:    "https://github.com/sisu-network/bentos-backend.git",
		},
		{
			name:           "https clone url",
			input:          "https://github.com/sisu-network/bentos-backend.git",
			wantRepository: "sisu-network/bentos-backend",
			wantRepoURL:    "https://github.com/sisu-network/bentos-backend.git",
		},
		{
			name:           "http clone url",
			input:          "http://github.com/sisu-network/bentos-backend.git",
			wantRepository: "sisu-network/bentos-backend",
			wantRepoURL:    "http://github.com/sisu-network/bentos-backend.git",
		},
		{
			name:           "ssh scp format",
			input:          "git@github.com:sisu-network/bentos-backend.git",
			wantRepository: "sisu-network/bentos-backend",
			wantRepoURL:    "git@github.com:sisu-network/bentos-backend.git",
		},
		{
			name:           "ssh url format",
			input:          "ssh://git@github.com/sisu-network/bentos-backend.git",
			wantRepository: "sisu-network/bentos-backend",
			wantRepoURL:    "ssh://git@github.com/sisu-network/bentos-backend.git",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			repository, repoURL, _, err := normalizeRepo(tc.input)

			require.NoError(t, err)
			require.Equal(t, tc.wantRepository, repository)
			require.Equal(t, tc.wantRepoURL, repoURL)
		})
	}
}

func TestNormalizeRepoRejectsInvalidValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "missing repo in ssh scp format",
			input: "git@github.com:sisu-network",
		},
		{
			name:  "extra path segments in ssh scp format",
			input: "git@github.com:sisu-network/bentos-backend/extra",
		},
		{
			name:  "non github https host",
			input: "https://example.com/sisu-network/bentos-backend.git",
		},
		{
			name:  "ssh url with non-git user",
			input: "ssh://alice@github.com/sisu-network/bentos-backend.git",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, _, _, err := normalizeRepo(tc.input)

			require.EqualError(t, err, `invalid --repo value "`+tc.input+`"`)
		})
	}
}

func TestNormalizeRepoEmptyValueReturnsEmptyResult(t *testing.T) {
	t.Parallel()

	repository, repoURL, _, err := normalizeRepo("   ")

	require.NoError(t, err)
	require.Empty(t, repository)
	require.Empty(t, repoURL)
}

type fakeChangeRequestUseCase struct {
	requests []usecase.ChangeRequestRequest
}

func (f *fakeChangeRequestUseCase) Execute(_ context.Context, request usecase.ChangeRequestRequest) (usecase.ChangeRequestExecutionResult, error) {
	f.requests = append(f.requests, request)
	return usecase.ChangeRequestExecutionResult{}, nil
}

type pullRequestInfoInput struct {
	repository string
	number     int
}

type fakeGitHubClient struct {
	resolvedRepository string
	resolveErr         error
	pullRequestInfo    githubvcs.PullRequestInfo
	pullRequestInfoErr error

	resolveRepositoryInputs []string
	pullRequestInfoInputs   []pullRequestInfoInput
}

func (f *fakeGitHubClient) ResolveRepository(_ context.Context, repository string) (string, error) {
	f.resolveRepositoryInputs = append(f.resolveRepositoryInputs, repository)
	if f.resolveErr != nil {
		return "", f.resolveErr
	}
	return f.resolvedRepository, nil
}

func (f *fakeGitHubClient) GetPullRequestInfo(_ context.Context, repository string, pullRequestNumber int) (githubvcs.PullRequestInfo, error) {
	f.pullRequestInfoInputs = append(f.pullRequestInfoInputs, pullRequestInfoInput{
		repository: repository,
		number:     pullRequestNumber,
	})
	if f.pullRequestInfoErr != nil {
		return githubvcs.PullRequestInfo{}, f.pullRequestInfoErr
	}
	return f.pullRequestInfo, nil
}

type commandTestSpyLogger struct {
	events []string
}

func (s *commandTestSpyLogger) Tracef(format string, args ...any) {
	s.events = append(s.events, "trace:"+fmt.Sprintf(format, args...))
}

func (s *commandTestSpyLogger) Debugf(format string, args ...any) {
	s.events = append(s.events, "debug:"+fmt.Sprintf(format, args...))
}

func (s *commandTestSpyLogger) Infof(format string, args ...any) {
	s.events = append(s.events, "info:"+fmt.Sprintf(format, args...))
}

func (s *commandTestSpyLogger) Warnf(format string, args ...any) {
	s.events = append(s.events, "warn:"+fmt.Sprintf(format, args...))
}

func (s *commandTestSpyLogger) Errorf(format string, args ...any) {
	s.events = append(s.events, "error:"+fmt.Sprintf(format, args...))
}

func containsLogEvent(events []string, target string) bool {
	for _, event := range events {
		if strings.Contains(event, target) {
			return true
		}
	}
	return false
}
