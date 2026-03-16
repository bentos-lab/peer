package cli

import (
	"context"

	"bentos-backend/domain"
)

type fakeGitHubClient struct {
	resolvedRepository string
	pullRequestInfo    domain.ChangeRequestInfo
	issue              domain.Issue
	issueComments      []domain.IssueComment
}

func (f *fakeGitHubClient) ResolveRepository(_ context.Context, _ string) (string, error) {
	return f.resolvedRepository, nil
}

func (f *fakeGitHubClient) GetPullRequestInfo(_ context.Context, _ string, _ int) (domain.ChangeRequestInfo, error) {
	return f.pullRequestInfo, nil
}

func (f *fakeGitHubClient) GetIssue(_ context.Context, _ string, _ int) (domain.Issue, error) {
	return f.issue, nil
}

func (f *fakeGitHubClient) ListIssueComments(_ context.Context, _ string, _ int) ([]domain.IssueComment, error) {
	return f.issueComments, nil
}

func (f *fakeGitHubClient) ListChangeRequestComments(_ context.Context, _ string, _ int) ([]domain.IssueComment, error) {
	return f.issueComments, nil
}

func (f *fakeGitHubClient) GetPullRequestReview(_ context.Context, _ string, _ int, _ int64) (domain.ReviewSummary, error) {
	return domain.ReviewSummary{}, nil
}

func (f *fakeGitHubClient) GetIssueComment(_ context.Context, _ string, _ int, _ int64) (domain.IssueComment, error) {
	return domain.IssueComment{}, nil
}

func (f *fakeGitHubClient) GetReviewComment(_ context.Context, _ string, _ int, _ int64) (domain.ReviewComment, error) {
	return domain.ReviewComment{}, nil
}

func (f *fakeGitHubClient) ListReviewComments(_ context.Context, _ string, _ int) ([]domain.ReviewComment, error) {
	return nil, nil
}

