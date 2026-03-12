package cli

import (
	"context"
	"testing"
	"time"

	githubvcs "bentos-backend/adapter/outbound/vcs/github"
	"bentos-backend/domain"
	"bentos-backend/usecase"
	"github.com/stretchr/testify/require"
)

type fakeReplyCommentUseCase struct {
	requests []usecase.ReplyCommentRequest
}

func (f *fakeReplyCommentUseCase) Execute(_ context.Context, request usecase.ReplyCommentRequest) (usecase.ReplyCommentResult, error) {
	f.requests = append(f.requests, request)
	return usecase.ReplyCommentResult{}, nil
}

type fakeReplyCommentGitHubClient struct {
	resolvedRepository string
	prInfo             githubvcs.PullRequestInfo
	resolveInputs      []string
	reviewComment      githubvcs.ReviewComment
	reviewComments     []githubvcs.ReviewComment
	issueComment       githubvcs.IssueComment
	reviewSummary      githubvcs.PullRequestReviewSummary
}

func (f *fakeReplyCommentGitHubClient) ResolveRepository(_ context.Context, repository string) (string, error) {
	f.resolveInputs = append(f.resolveInputs, repository)
	return f.resolvedRepository, nil
}

func (f *fakeReplyCommentGitHubClient) GetPullRequestInfo(_ context.Context, _ string, _ int) (githubvcs.PullRequestInfo, error) {
	return f.prInfo, nil
}

func (f *fakeReplyCommentGitHubClient) GetPullRequestReview(_ context.Context, _ string, _ int, _ int64) (githubvcs.PullRequestReviewSummary, error) {
	return f.reviewSummary, nil
}

func (f *fakeReplyCommentGitHubClient) GetIssueComment(_ context.Context, _ string, _ int64) (githubvcs.IssueComment, error) {
	return f.issueComment, nil
}

func (f *fakeReplyCommentGitHubClient) GetReviewComment(_ context.Context, _ string, _ int64) (githubvcs.ReviewComment, error) {
	return f.reviewComment, nil
}

func (f *fakeReplyCommentGitHubClient) ListIssueComments(_ context.Context, _ string, _ int) ([]githubvcs.IssueComment, error) {
	return nil, nil
}

func (f *fakeReplyCommentGitHubClient) ListReviewComments(_ context.Context, _ string, _ int) ([]githubvcs.ReviewComment, error) {
	return f.reviewComments, nil
}

func TestReplyCommentCommandRejectsQuestionWithCommentFlag(t *testing.T) {
	useCase := &fakeReplyCommentUseCase{}
	client := &fakeReplyCommentGitHubClient{}
	command := NewReplyCommentCommand(useCase, client, "autogitbot")

	err := command.Run(context.Background(), ReplyCommentRunParams{
		VCSProvider:   "github",
		Repo:          "owner/repo",
		ChangeRequest: "7",
		Question:      "What changed?",
		Comment:       true,
	})
	require.Error(t, err)
	require.Empty(t, useCase.requests)
}

func TestReplyCommentCommandQuestionBuildsInlineThread(t *testing.T) {
	useCase := &fakeReplyCommentUseCase{}
	client := &fakeReplyCommentGitHubClient{
		resolvedRepository: "owner/repo",
		prInfo: githubvcs.PullRequestInfo{
			Repository:  "owner/repo",
			Number:      7,
			Title:       "title",
			Description: "description",
			BaseRef:     "main",
			HeadRef:     "feature",
		},
	}
	command := NewReplyCommentCommand(useCase, client, "autogitbot")

	err := command.Run(context.Background(), ReplyCommentRunParams{
		VCSProvider:   "github",
		ChangeRequest: "7",
		Question:      "What changed?",
	})
	require.NoError(t, err)
	require.Len(t, useCase.requests, 1)
	request := useCase.requests[0]
	require.Equal(t, "owner/repo", request.Repository)
	require.Equal(t, "", request.RepoURL)
	require.Equal(t, 7, request.ChangeRequestNumber)
	require.Equal(t, domain.CommentKindIssue, request.CommentKind)
	require.False(t, request.Publish)
	require.Equal(t, "What changed?", request.Question)
	require.Len(t, request.Thread.Comments, 1)
	require.WithinDuration(t, time.Now(), request.Thread.Comments[0].CreatedAt, time.Second)
	require.NotEmpty(t, request.Thread.Context)
	require.Equal(t, []string{""}, client.resolveInputs)
}

func TestReplyCommentCommandWithRepoSetsRepoURL(t *testing.T) {
	useCase := &fakeReplyCommentUseCase{}
	client := &fakeReplyCommentGitHubClient{
		resolvedRepository: "owner/repo",
		prInfo: githubvcs.PullRequestInfo{
			Repository:  "owner/repo",
			Number:      7,
			Title:       "title",
			Description: "description",
			BaseRef:     "main",
			HeadRef:     "feature",
		},
	}
	command := NewReplyCommentCommand(useCase, client, "autogitbot")

	err := command.Run(context.Background(), ReplyCommentRunParams{
		VCSProvider:   "github",
		Repo:          "owner/repo",
		ChangeRequest: "7",
		Question:      "What changed?",
	})
	require.NoError(t, err)
	require.Len(t, useCase.requests, 1)
	require.Equal(t, "https://github.com/owner/repo.git", useCase.requests[0].RepoURL)
	require.Equal(t, []string{"owner/repo"}, client.resolveInputs)
}

func TestReplyCommentCommandParsesDiscussionAnchor(t *testing.T) {
	useCase := &fakeReplyCommentUseCase{}
	client := &fakeReplyCommentGitHubClient{
		resolvedRepository: "owner/repo",
		prInfo: githubvcs.PullRequestInfo{
			Repository:  "owner/repo",
			Number:      7,
			Title:       "title",
			Description: "description",
			BaseRef:     "main",
			HeadRef:     "feature",
		},
		reviewComment: githubvcs.ReviewComment{
			ID:       2909490245,
			Path:     "adapter/file.go",
			DiffHunk: "@@ -1 +1 @@\n- old\n+ new",
			Line:     12,
		},
		reviewComments: []githubvcs.ReviewComment{{
			ID:       2909490245,
			Path:     "adapter/file.go",
			DiffHunk: "@@ -1 +1 @@\n- old\n+ new",
			Line:     12,
		}},
		reviewSummary: githubvcs.PullRequestReviewSummary{
			Body:  "LGTM",
			State: "APPROVED",
			User:  githubvcs.CommentAuthor{Login: "reviewer"},
		},
	}
	command := NewReplyCommentCommand(useCase, client, "autogitbot")

	err := command.Run(context.Background(), ReplyCommentRunParams{
		VCSProvider:   "github",
		ChangeRequest: "7",
		CommentID:     "discussion_r2909490245",
		Comment:       true,
	})
	require.NoError(t, err)
	require.Len(t, useCase.requests, 1)
	require.Equal(t, int64(2909490245), useCase.requests[0].CommentID)
	require.NotEmpty(t, useCase.requests[0].Thread.Context)
}

func TestReplyCommentCommandParsesIssueCommentAnchor(t *testing.T) {
	useCase := &fakeReplyCommentUseCase{}
	client := &fakeReplyCommentGitHubClient{
		resolvedRepository: "owner/repo",
		prInfo: githubvcs.PullRequestInfo{
			Repository:  "owner/repo",
			Number:      7,
			Title:       "title",
			Description: "description",
			BaseRef:     "main",
			HeadRef:     "feature",
		},
		issueComment: githubvcs.IssueComment{
			ID: 12345,
		},
	}
	command := NewReplyCommentCommand(useCase, client, "autogitbot")

	err := command.Run(context.Background(), ReplyCommentRunParams{
		VCSProvider:   "github",
		ChangeRequest: "7",
		CommentID:     "issuecomment-12345",
		Comment:       true,
	})
	require.NoError(t, err)
	require.Len(t, useCase.requests, 1)
	require.Equal(t, int64(12345), useCase.requests[0].CommentID)
}
