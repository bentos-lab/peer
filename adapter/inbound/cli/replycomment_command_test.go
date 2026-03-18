package cli

import (
	"context"
	"testing"
	"time"

	"bentos-backend/config"
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
	prInfo             domain.ChangeRequestInfo
	resolveInputs      []string
	reviewComment      domain.ReviewComment
	reviewComments     []domain.ReviewComment
	issue              domain.Issue
	issueComment       domain.IssueComment
	reviewSummary      domain.ReviewSummary
}

func (f *fakeReplyCommentGitHubClient) ResolveRepository(_ context.Context, repository string) (string, error) {
	f.resolveInputs = append(f.resolveInputs, repository)
	return f.resolvedRepository, nil
}

func (f *fakeReplyCommentGitHubClient) GetPullRequestInfo(_ context.Context, _ string, _ int) (domain.ChangeRequestInfo, error) {
	return f.prInfo, nil
}

func (f *fakeReplyCommentGitHubClient) GetIssue(_ context.Context, _ string, _ int) (domain.Issue, error) {
	return f.issue, nil
}

func (f *fakeReplyCommentGitHubClient) GetPullRequestReview(_ context.Context, _ string, _ int, _ int64) (domain.ReviewSummary, error) {
	return f.reviewSummary, nil
}

func (f *fakeReplyCommentGitHubClient) GetIssueComment(_ context.Context, _ string, _ int, _ int64) (domain.IssueComment, error) {
	return f.issueComment, nil
}

func (f *fakeReplyCommentGitHubClient) GetReviewComment(_ context.Context, _ string, _ int, _ int64) (domain.ReviewComment, error) {
	return f.reviewComment, nil
}

func (f *fakeReplyCommentGitHubClient) ListIssueComments(_ context.Context, _ string, _ int) ([]domain.IssueComment, error) {
	return nil, nil
}

func (f *fakeReplyCommentGitHubClient) ListChangeRequestComments(_ context.Context, _ string, _ int) ([]domain.IssueComment, error) {
	return nil, nil
}

func (f *fakeReplyCommentGitHubClient) ListReviewComments(_ context.Context, _ string, _ int) ([]domain.ReviewComment, error) {
	return f.reviewComments, nil
}

func TestReplyCommentCommandRejectsQuestionWithPublishFlag(t *testing.T) {
	useCase := &fakeReplyCommentUseCase{}
	client := &fakeReplyCommentGitHubClient{}
	builder := func(_ string) (usecase.ReplyCommentUseCase, error) {
		return useCase, nil
	}
	resolver := StaticVCSClients{GitHub: client}
	command := NewReplyCommentCommand(builder, resolver, &testCodeEnvironmentFactory{}, &testRecipeLoader{}, "peerbot", nil)

	err := command.Run(context.Background(), config.Config{}, ReplyCommentRunParams{
		VCSProvider:   "github",
		Repo:          "owner/repo",
		ChangeRequest: "7",
		Question:      "What changed?",
		Publish:       true,
	})
	require.Error(t, err)
	require.Empty(t, useCase.requests)
}

func TestReplyCommentCommandQuestionBuildsInlineThread(t *testing.T) {
	useCase := &fakeReplyCommentUseCase{}
	client := &fakeReplyCommentGitHubClient{
		resolvedRepository: "owner/repo",
		prInfo: domain.ChangeRequestInfo{
			Repository:  "owner/repo",
			Number:      7,
			Title:       "title",
			Description: "description",
			BaseRef:     "main",
			HeadRef:     "feature",
		},
	}
	builder := func(_ string) (usecase.ReplyCommentUseCase, error) {
		return useCase, nil
	}
	resolver := StaticVCSClients{GitHub: client}
	command := NewReplyCommentCommand(builder, resolver, &testCodeEnvironmentFactory{}, &testRecipeLoader{}, "peerbot", nil)

	err := command.Run(context.Background(), config.Config{}, ReplyCommentRunParams{
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
		prInfo: domain.ChangeRequestInfo{
			Repository:  "owner/repo",
			Number:      7,
			Title:       "title",
			Description: "description",
			BaseRef:     "main",
			HeadRef:     "feature",
		},
	}
	builder := func(_ string) (usecase.ReplyCommentUseCase, error) {
		return useCase, nil
	}
	resolver := StaticVCSClients{GitHub: client}
	command := NewReplyCommentCommand(builder, resolver, &testCodeEnvironmentFactory{}, &testRecipeLoader{}, "peerbot", nil)

	err := command.Run(context.Background(), config.Config{}, ReplyCommentRunParams{
		VCSProvider:   "github",
		Repo:          "https://github.com/owner/repo.git",
		ChangeRequest: "7",
		Question:      "What changed?",
	})
	require.NoError(t, err)
	require.Len(t, useCase.requests, 1)
	require.Equal(t, "https://github.com/owner/repo.git", useCase.requests[0].RepoURL)
	require.Equal(t, []string{"owner/repo"}, client.resolveInputs)
}

func TestReplyCommentCommandUsesReviewThreadWhenReviewCommentResolved(t *testing.T) {
	useCase := &fakeReplyCommentUseCase{}
	client := &fakeReplyCommentGitHubClient{
		resolvedRepository: "owner/repo",
		prInfo: domain.ChangeRequestInfo{
			Repository:  "owner/repo",
			Number:      7,
			Title:       "title",
			Description: "description",
			BaseRef:     "main",
			HeadRef:     "feature",
		},
		reviewComment: domain.ReviewComment{
			ID:          123,
			Body:        "@peerbot Can you clarify this?",
			InReplyToID: 111,
			Path:        "main.go",
			DiffHunk:    "diff",
			Line:        42,
			Side:        "RIGHT",
			ReviewID:    9,
			CreatedAt:   time.Now().Add(-time.Minute),
		},
		reviewComments: []domain.ReviewComment{{
			ID:        111,
			Body:      "Initial comment",
			Path:      "main.go",
			DiffHunk:  "diff",
			Line:      42,
			Side:      "RIGHT",
			ReviewID:  9,
			CreatedAt: time.Now().Add(-time.Hour),
		}, {
			ID:          123,
			Body:        "@peerbot Can you clarify this?",
			InReplyToID: 111,
			Path:        "main.go",
			DiffHunk:    "diff",
			Line:        42,
			Side:        "RIGHT",
			ReviewID:    9,
			CreatedAt:   time.Now().Add(-time.Minute),
		}},
		reviewSummary: domain.ReviewSummary{
			ID:    9,
			Body:  "Review body",
			State: "commented",
			User:  domain.CommentAuthor{Login: "reviewer"},
		},
	}
	builder := func(_ string) (usecase.ReplyCommentUseCase, error) {
		return useCase, nil
	}
	resolver := StaticVCSClients{GitHub: client}
	command := NewReplyCommentCommand(builder, resolver, &testCodeEnvironmentFactory{}, &testRecipeLoader{}, "peerbot", nil)

	err := command.Run(context.Background(), config.Config{}, ReplyCommentRunParams{
		VCSProvider:   "github",
		ChangeRequest: "7",
		CommentID:     "123",
	})
	require.NoError(t, err)
	require.Len(t, useCase.requests, 1)
	request := useCase.requests[0]
	require.Equal(t, domain.CommentKindReview, request.CommentKind)
	require.Equal(t, 111, int(request.Thread.RootID))
	require.Equal(t, "Can you clarify this?", request.Question)
	require.Len(t, request.Thread.Comments, 2)
}

func TestReplyCommentCommandParsesIssueCommentAnchor(t *testing.T) {
	useCase := &fakeReplyCommentUseCase{}
	client := &fakeReplyCommentGitHubClient{
		resolvedRepository: "owner/repo",
		prInfo: domain.ChangeRequestInfo{
			Repository:  "owner/repo",
			Number:      7,
			Title:       "title",
			Description: "description",
			BaseRef:     "main",
			HeadRef:     "feature",
		},
		issueComment: domain.IssueComment{
			ID:        222,
			Body:      "@peerbot Please explain",
			CreatedAt: time.Now().Add(-time.Minute),
		},
	}
	builder := func(_ string) (usecase.ReplyCommentUseCase, error) {
		return useCase, nil
	}
	resolver := StaticVCSClients{GitHub: client}
	command := NewReplyCommentCommand(builder, resolver, &testCodeEnvironmentFactory{}, &testRecipeLoader{}, "peerbot", nil)

	err := command.Run(context.Background(), config.Config{}, ReplyCommentRunParams{
		VCSProvider:   "github",
		ChangeRequest: "7",
		CommentID:     "issuecomment-222",
	})
	require.NoError(t, err)
	require.Len(t, useCase.requests, 1)
	request := useCase.requests[0]
	require.Equal(t, domain.CommentKindIssue, request.CommentKind)
	require.Equal(t, "Please explain", request.Question)
}
