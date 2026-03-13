package cli

import (
	"context"

	githubvcs "bentos-backend/adapter/outbound/vcs/github"
)

type fakeGitHubClient struct {
	resolvedRepository string
	pullRequestInfo    githubvcs.PullRequestInfo
	issue              githubvcs.Issue
	issueComments      []githubvcs.IssueComment
}

func (f *fakeGitHubClient) ResolveRepository(_ context.Context, _ string) (string, error) {
	return f.resolvedRepository, nil
}

func (f *fakeGitHubClient) GetPullRequestInfo(_ context.Context, _ string, _ int) (githubvcs.PullRequestInfo, error) {
	return f.pullRequestInfo, nil
}

func (f *fakeGitHubClient) GetIssue(_ context.Context, _ string, _ int) (githubvcs.Issue, error) {
	return f.issue, nil
}

func (f *fakeGitHubClient) ListIssueComments(_ context.Context, _ string, _ int) ([]githubvcs.IssueComment, error) {
	return f.issueComments, nil
}
