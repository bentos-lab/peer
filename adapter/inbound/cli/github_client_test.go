package cli

import (
	"context"

	githubvcs "bentos-backend/adapter/outbound/vcs/github"
)

type fakeGitHubClient struct {
	resolvedRepository string
	pullRequestInfo    githubvcs.PullRequestInfo
}

func (f *fakeGitHubClient) ResolveRepository(_ context.Context, _ string) (string, error) {
	return f.resolvedRepository, nil
}

func (f *fakeGitHubClient) GetPullRequestInfo(_ context.Context, _ string, _ int) (githubvcs.PullRequestInfo, error) {
	return f.pullRequestInfo, nil
}
