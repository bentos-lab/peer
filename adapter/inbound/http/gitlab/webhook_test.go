package gitlab

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"bentos-backend/domain"
	"bentos-backend/shared/jobqueue"
	"bentos-backend/usecase"
	uccontracts "bentos-backend/usecase/contracts"
	"github.com/stretchr/testify/require"
)

type testGitLabClient struct {
	prInfo domain.ChangeRequestInfo
	issue  domain.Issue
}

func (c *testGitLabClient) GetPullRequestInfo(_ context.Context, _ string, _ int) (domain.ChangeRequestInfo, error) {
	return c.prInfo, nil
}

func (c *testGitLabClient) GetIssue(_ context.Context, _ string, _ int) (domain.Issue, error) {
	return c.issue, nil
}

func (c *testGitLabClient) ListIssueComments(_ context.Context, _ string, _ int) ([]domain.IssueComment, error) {
	return []domain.IssueComment{}, nil
}

func (c *testGitLabClient) ListChangeRequestComments(_ context.Context, _ string, _ int) ([]domain.IssueComment, error) {
	return []domain.IssueComment{{ID: 1, Body: "hello", Author: domain.CommentAuthor{Login: "user"}, CreatedAt: time.Now()}}, nil
}

func (c *testGitLabClient) ListReviewComments(_ context.Context, _ string, _ int) ([]domain.ReviewComment, error) {
	return []domain.ReviewComment{}, nil
}

func (c *testGitLabClient) GetPullRequestReview(_ context.Context, _ string, _ int, reviewID int64) (domain.ReviewSummary, error) {
	return domain.ReviewSummary{ID: reviewID}, nil
}

func (c *testGitLabClient) BuildAuthenticatedCloneURL(_ string) (string, error) {
	return "https://oauth2:token@gitlab.example.com/group/repo.git", nil
}

type testEnvFactory struct{}

func (f *testEnvFactory) New(_ context.Context, _ domain.CodeEnvironmentInitOptions) (uccontracts.CodeEnvironment, error) {
	return &testEnvironment{}, nil
}

type testEnvironment struct{}

func (e *testEnvironment) SetupAgent(_ context.Context, _ domain.CodingAgentSetupOptions) (uccontracts.CodingAgent, error) {
	return nil, nil
}

func (e *testEnvironment) LoadChangedFiles(_ context.Context, _ domain.CodeEnvironmentLoadOptions) ([]domain.ChangedFile, error) {
	return nil, nil
}

func (e *testEnvironment) ReadFile(_ context.Context, _ string, _ string) (string, bool, error) {
	return "", false, nil
}

func (e *testEnvironment) PushChanges(_ context.Context, _ domain.CodeEnvironmentPushOptions) (domain.CodeEnvironmentPushResult, error) {
	return domain.CodeEnvironmentPushResult{}, nil
}

func (e *testEnvironment) Cleanup(_ context.Context) error {
	return nil
}

type testRecipeLoader struct{}

func (l *testRecipeLoader) Load(_ context.Context, _ uccontracts.CodeEnvironment, _ string) (domain.CustomRecipe, error) {
	return domain.CustomRecipe{}, nil
}

type testReviewUseCase struct {
	called chan usecase.ReviewRequest
}

func (t *testReviewUseCase) Execute(_ context.Context, request usecase.ReviewRequest) (usecase.ReviewExecutionResult, error) {
	t.called <- request
	return usecase.ReviewExecutionResult{}, nil
}

type testReplyUseCase struct {
	called chan usecase.ReplyCommentRequest
}

func (t *testReplyUseCase) Execute(_ context.Context, request usecase.ReplyCommentRequest) (usecase.ReplyCommentResult, error) {
	t.called <- request
	return usecase.ReplyCommentResult{}, nil
}

func TestGitLabWebhookMergeRequestTriggersReview(t *testing.T) {
	called := make(chan usecase.ReviewRequest, 1)
	client := &testGitLabClient{
		prInfo: domain.ChangeRequestInfo{
			Repository:  "group/repo",
			Number:      12,
			Title:       "title",
			Description: "desc",
			BaseRef:     "base",
			HeadRef:     "head",
			HeadRefName: "feature",
		},
	}
	handler := NewHandler(
		func(string) (usecase.ReviewUseCase, error) { return &testReviewUseCase{called: called}, nil },
		nil,
		nil,
		nil,
		client,
		nil,
		&testEnvFactory{},
		&testRecipeLoader{},
		nil,
		"secret",
		"autogitbot",
		true,
		[]string{"opened"},
		false,
		false,
		nil,
		false,
		false,
		nil,
		false,
		false,
		false,
		nil,
		nil,
		jobqueue.NewManager(1),
	)

	payload := []byte(`{"object_kind":"merge_request","project":{"id":1,"path_with_namespace":"group/repo"},"object_attributes":{"iid":12,"action":"open"}}`)
	req := httptest.NewRequest(http.MethodPost, "/webhook/gitlab", bytes.NewReader(payload))
	req.Header.Set("X-Gitlab-Event", "Merge Request Hook")
	req.Header.Set("X-Gitlab-Token", "secret")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusAccepted, rec.Code)

	select {
	case <-called:
	case <-time.After(2 * time.Second):
		t.Fatalf("expected review usecase to run")
	}
}

func TestGitLabWebhookNoteTriggersReplyComment(t *testing.T) {
	called := make(chan usecase.ReplyCommentRequest, 1)
	client := &testGitLabClient{
		prInfo: domain.ChangeRequestInfo{
			Repository:  "group/repo",
			Number:      7,
			Title:       "title",
			Description: "desc",
			BaseRef:     "base",
			HeadRef:     "head",
			HeadRefName: "feature",
		},
	}
	handler := NewHandler(
		nil,
		nil,
		nil,
		func(string) (usecase.ReplyCommentUseCase, error) { return &testReplyUseCase{called: called}, nil },
		client,
		nil,
		&testEnvFactory{},
		&testRecipeLoader{},
		nil,
		"secret",
		"autogitbot",
		false,
		nil,
		false,
		false,
		nil,
		false,
		false,
		nil,
		false,
		false,
		true,
		[]string{"note"},
		[]string{"created"},
		jobqueue.NewManager(1),
	)

	payload := []byte(`{"object_kind":"note","user":{"username":"alice"},"project":{"id":1,"path_with_namespace":"group/repo"},"object_attributes":{"id":99,"note":"@autogitbot please help","noteable_type":"MergeRequest","action":"created"},"merge_request":{"iid":7}}`)
	req := httptest.NewRequest(http.MethodPost, "/webhook/gitlab", bytes.NewReader(payload))
	req.Header.Set("X-Gitlab-Event", "Note Hook")
	req.Header.Set("X-Gitlab-Token", "secret")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusAccepted, rec.Code)

	select {
	case <-called:
	case <-time.After(2 * time.Second):
		t.Fatalf("expected replycomment usecase to run")
	}
}
