package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	githubvcs "github.com/bentos-lab/peer/adapter/outbound/vcs/github"
	"github.com/bentos-lab/peer/domain"
	"github.com/bentos-lab/peer/shared/jobqueue"
	"github.com/bentos-lab/peer/usecase"
	uccontracts "github.com/bentos-lab/peer/usecase/contracts"
	"github.com/stretchr/testify/require"
)

const testWebhookSecret = "test-secret"

var defaultTestReviewEvents = []string{"opened", "synchronize", "reopened"}
var defaultTestOverviewEvents = []string{"opened"}
var defaultTestAutogenEvents = []string{"opened", "reopened", "synchronize"}
var defaultTestReplyEvents = []string{"issue_comment", "pull_request_review_comment"}
var defaultTestReplyActions = []string{"created"}

type mockReviewUseCase struct {
	requestCh chan usecase.ReviewRequest
	ctxCh     chan context.Context
	proceedCh chan struct{}
	err       error
	panicVal  any
}

func (m *mockReviewUseCase) Execute(ctx context.Context, request usecase.ReviewRequest) (usecase.ReviewExecutionResult, error) {
	if m.panicVal != nil {
		panic(m.panicVal)
	}
	if m.ctxCh != nil {
		m.ctxCh <- ctx
	}
	if m.requestCh != nil {
		m.requestCh <- request
	}
	if m.proceedCh != nil {
		<-m.proceedCh
	}
	return usecase.ReviewExecutionResult{}, m.err
}

type mockOverviewUseCase struct {
	requestCh chan usecase.OverviewRequest
	ctxCh     chan context.Context
	proceedCh chan struct{}
	err       error
}

func (m *mockOverviewUseCase) Execute(ctx context.Context, request usecase.OverviewRequest) (usecase.OverviewExecutionResult, error) {
	if m.ctxCh != nil {
		m.ctxCh <- ctx
	}
	if m.requestCh != nil {
		m.requestCh <- request
	}
	if m.proceedCh != nil {
		<-m.proceedCh
	}
	return usecase.OverviewExecutionResult{}, m.err
}

type mockAutogenUseCase struct {
	requestCh chan usecase.AutogenRequest
	err       error
}

func (m *mockAutogenUseCase) Execute(_ context.Context, request usecase.AutogenRequest) (usecase.AutogenExecutionResult, error) {
	if m.requestCh != nil {
		m.requestCh <- request
	}
	return usecase.AutogenExecutionResult{}, m.err
}

type mockReplyCommentUseCase struct {
	requestCh chan usecase.ReplyCommentRequest
}

func (m *mockReplyCommentUseCase) Execute(_ context.Context, request usecase.ReplyCommentRequest) (usecase.ReplyCommentResult, error) {
	if m.requestCh != nil {
		m.requestCh <- request
	}
	return usecase.ReplyCommentResult{}, nil
}

func newReviewBuilder(uc usecase.ReviewUseCase) ReviewUseCaseBuilder {
	return func(_ string) (usecase.ReviewUseCase, error) {
		return uc, nil
	}
}

func newOverviewBuilder(uc usecase.OverviewUseCase) OverviewUseCaseBuilder {
	return func(_ string) (usecase.OverviewUseCase, error) {
		return uc, nil
	}
}

func newAutogenBuilder(uc usecase.AutogenUseCase) AutogenUseCaseBuilder {
	return func(_ string) (usecase.AutogenUseCase, error) {
		return uc, nil
	}
}

type spyLogger struct {
	events []string
}

type mockInstallationTokenProvider struct {
	token string
	err   error
}

func (m mockInstallationTokenProvider) GetInstallationAccessToken(_ context.Context, _ string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.token, nil
}

func (m mockInstallationTokenProvider) GetPullRequestInfo(_ context.Context, _ string, _ int) (domain.ChangeRequestInfo, error) {
	return domain.ChangeRequestInfo{}, errors.New("not implemented")
}

func (m mockInstallationTokenProvider) GetIssue(_ context.Context, _ string, _ int) (domain.Issue, error) {
	return domain.Issue{}, errors.New("not implemented")
}

func (m mockInstallationTokenProvider) GetPullRequestReview(_ context.Context, _ string, _ int, _ int64) (domain.ReviewSummary, error) {
	return domain.ReviewSummary{}, errors.New("not implemented")
}

func (m mockInstallationTokenProvider) ListIssueComments(_ context.Context, _ string, _ int) ([]domain.IssueComment, error) {
	return nil, errors.New("not implemented")
}

func (m mockInstallationTokenProvider) ListReviewComments(_ context.Context, _ string, _ int) ([]domain.ReviewComment, error) {
	return nil, errors.New("not implemented")
}

type issueAlignmentTokenProvider struct {
	token        string
	issueCalls   int
	commentCalls int
}

func (m *issueAlignmentTokenProvider) GetInstallationAccessToken(_ context.Context, _ string) (string, error) {
	return m.token, nil
}

func (m *issueAlignmentTokenProvider) GetPullRequestInfo(_ context.Context, _ string, _ int) (domain.ChangeRequestInfo, error) {
	return domain.ChangeRequestInfo{}, errors.New("not implemented")
}

func (m *issueAlignmentTokenProvider) GetIssue(_ context.Context, repository string, issueNumber int) (domain.Issue, error) {
	m.issueCalls++
	return domain.Issue{Repository: repository, Number: issueNumber, Title: "Issue", Body: "Body"}, nil
}

func (m *issueAlignmentTokenProvider) GetPullRequestReview(_ context.Context, _ string, _ int, _ int64) (domain.ReviewSummary, error) {
	return domain.ReviewSummary{}, errors.New("not implemented")
}

func (m *issueAlignmentTokenProvider) ListIssueComments(_ context.Context, _ string, _ int) ([]domain.IssueComment, error) {
	m.commentCalls++
	return nil, nil
}

func (m *issueAlignmentTokenProvider) ListReviewComments(_ context.Context, _ string, _ int) ([]domain.ReviewComment, error) {
	return nil, errors.New("not implemented")
}

type replyCommentTokenProvider struct {
	token  string
	prInfo domain.ChangeRequestInfo
}

func (m *replyCommentTokenProvider) GetInstallationAccessToken(_ context.Context, _ string) (string, error) {
	return m.token, nil
}

func (m *replyCommentTokenProvider) GetPullRequestInfo(_ context.Context, _ string, _ int) (domain.ChangeRequestInfo, error) {
	return m.prInfo, nil
}

func (m *replyCommentTokenProvider) GetIssue(_ context.Context, _ string, _ int) (domain.Issue, error) {
	return domain.Issue{}, errors.New("not implemented")
}

func (m *replyCommentTokenProvider) GetPullRequestReview(_ context.Context, _ string, _ int, _ int64) (domain.ReviewSummary, error) {
	return domain.ReviewSummary{}, errors.New("not implemented")
}

func (m *replyCommentTokenProvider) ListIssueComments(_ context.Context, _ string, _ int) ([]domain.IssueComment, error) {
	return nil, nil
}

func (m *replyCommentTokenProvider) ListReviewComments(_ context.Context, _ string, _ int) ([]domain.ReviewComment, error) {
	return nil, nil
}

type mockRecipeConfigLoader struct {
	recipe      domain.CustomRecipe
	err         error
	lastRepoURL string
	lastHeadRef string
}

func (m *mockRecipeConfigLoader) Load(_ context.Context, repoURL string, headRef string) (domain.CustomRecipe, error) {
	m.lastRepoURL = repoURL
	m.lastHeadRef = headRef
	if m.err != nil {
		return domain.CustomRecipe{}, m.err
	}
	return m.recipe, nil
}

type mockCodeEnvironment struct{}

func (e *mockCodeEnvironment) SetupAgent(_ context.Context, _ domain.CodingAgentSetupOptions) (uccontracts.CodingAgent, error) {
	return nil, nil
}

func (e *mockCodeEnvironment) LoadChangedFiles(_ context.Context, _ domain.CodeEnvironmentLoadOptions) ([]domain.ChangedFile, error) {
	return nil, nil
}

func (e *mockCodeEnvironment) ReadFile(_ context.Context, _ string, _ string) (string, bool, error) {
	return "", false, nil
}

func (e *mockCodeEnvironment) CommitChanges(_ context.Context, _ domain.CodeEnvironmentCommitOptions) (domain.CodeEnvironmentCommitResult, error) {
	return domain.CodeEnvironmentCommitResult{}, nil
}

func (e *mockCodeEnvironment) PushChanges(_ context.Context, _ domain.CodeEnvironmentPushOptions) (domain.CodeEnvironmentPushResult, error) {
	return domain.CodeEnvironmentPushResult{}, nil
}

func (e *mockCodeEnvironment) Cleanup(_ context.Context) error {
	return nil
}

type mockCodeEnvironmentFactory struct {
	env uccontracts.CodeEnvironment
	err error
}

func (f *mockCodeEnvironmentFactory) New(_ context.Context, _ domain.CodeEnvironmentInitOptions) (uccontracts.CodeEnvironment, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.env == nil {
		f.env = &mockCodeEnvironment{}
	}
	return f.env, nil
}

type mockRecipeLoader struct {
	recipe domain.CustomRecipe
	err    error
}

func (l *mockRecipeLoader) Load(_ context.Context, _ uccontracts.CodeEnvironment, _ string) (domain.CustomRecipe, error) {
	if l.err != nil {
		return domain.CustomRecipe{}, l.err
	}
	return l.recipe, nil
}

func newTestHandler(
	reviewBuilder ReviewUseCaseBuilder,
	overviewBuilder OverviewUseCaseBuilder,
	autogenBuilder AutogenUseCaseBuilder,
	replyCommentBuilder ReplyCommentUseCaseBuilder,
	tokenProvider CommentClient,
	recipeConfigLoader RecipeConfigLoader,
	logger usecase.Logger,
	webhookSecret string,
	replyTriggerName string,
	enableOverview bool,
	enableSuggestions bool,
) *Handler {
	return NewHandler(
		reviewBuilder,
		overviewBuilder,
		autogenBuilder,
		replyCommentBuilder,
		tokenProvider,
		recipeConfigLoader,
		&mockCodeEnvironmentFactory{},
		&mockRecipeLoader{},
		logger,
		webhookSecret,
		replyTriggerName,
		true,
		defaultTestReviewEvents,
		enableSuggestions,
		enableOverview,
		defaultTestOverviewEvents,
		true,
		false,
		defaultTestAutogenEvents,
		false,
		false,
		true,
		defaultTestReplyEvents,
		defaultTestReplyActions,
		jobqueue.NewManager(1),
	)
}

func (s *spyLogger) Tracef(format string, args ...any) {
	s.events = append(s.events, "trace:"+fmt.Sprintf(format, args...))
}

func (s *spyLogger) Infof(format string, args ...any) {
	s.events = append(s.events, "info:"+fmt.Sprintf(format, args...))
}

func (s *spyLogger) Debugf(format string, args ...any) {
	s.events = append(s.events, "debug:"+fmt.Sprintf(format, args...))
}

func (s *spyLogger) Warnf(format string, args ...any) {
	s.events = append(s.events, "warn:"+fmt.Sprintf(format, args...))
}

func (s *spyLogger) Errorf(format string, args ...any) {
	s.events = append(s.events, "error:"+fmt.Sprintf(format, args...))
}

func boolPointer(value bool) *bool {
	return &value
}

func TestHandler_ServeHTTP_ValidPayloadReturnsAcceptedAndMapsRequest(t *testing.T) {
	reviewUC := &mockReviewUseCase{
		requestCh: make(chan usecase.ReviewRequest, 1),
		ctxCh:     make(chan context.Context, 1),
	}
	overviewUC := &mockOverviewUseCase{
		requestCh: make(chan usecase.OverviewRequest, 1),
	}
	logger := &spyLogger{}
	handler := newTestHandler(newReviewBuilder(reviewUC), newOverviewBuilder(overviewUC), nil, nil, mockInstallationTokenProvider{token: "token-1"}, nil, logger, testWebhookSecret, "peerbot", true, true)
	payload := `{
		"action":"opened",
		"installation":{"id":123},
		"repository":{"full_name":"org/repo","clone_url":"https://github.com/org/repo.git"},
		"pull_request":{
			"number": 7,
			"title":"secret-title-token",
			"body":"secret-body-token",
			"base":{"ref":"main"},
			"head":{"ref":"feature"}
		}
	}`
	req := signedRequest(t, payload, "pull_request", testWebhookSecret)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	require.Equal(t, http.StatusAccepted, resp.Code)

	select {
	case reviewRequest := <-reviewUC.requestCh:
		require.Equal(t, "org/repo", reviewRequest.Input.Target.Repository)
		require.Equal(t, "https://x-access-token:token-1@github.com/org/repo.git", reviewRequest.Input.RepoURL)
		require.Equal(t, 7, reviewRequest.Input.Target.ChangeRequestNumber)
		require.Equal(t, "main", reviewRequest.Input.Base)
		require.Equal(t, "feature", reviewRequest.Input.Head)
		require.Equal(t, "opened", reviewRequest.Input.Metadata["action"])
		require.True(t, reviewRequest.Suggestions)
	case <-time.After(time.Second):
		t.Fatal("expected review usecase execution")
	}

	select {
	case reviewContext := <-reviewUC.ctxCh:
		require.Equal(t, "123", githubvcs.InstallationIDFromContext(reviewContext))
	case <-time.After(time.Second):
		t.Fatal("expected review usecase context")
	}

	require.Eventually(t, func() bool {
		return containsEvent(logger.events, `info:Pre-usecase input snapshot source="webhook" action="opened" repository="org/repo" changeRequestNumber=7 suggestions=true.`)
	}, time.Second, 10*time.Millisecond)
	require.Eventually(t, func() bool {
		return containsEvent(logger.events, `debug:Pre-usecase input details source="webhook" action="opened" base="main" head="feature"`)
	}, time.Second, 10*time.Millisecond)
	require.Eventually(t, func() bool {
		return containsEvent(logger.events, `repoURLPresent=true repoURLSafe="https://github.com/org/repo.git"`)
	}, time.Second, 10*time.Millisecond)
	require.False(t, containsEvent(logger.events, "x-access-token:token-1@"))
	require.False(t, containsEvent(logger.events, "secret-title-token"))
	require.False(t, containsEvent(logger.events, "secret-body-token"))
}

func TestHandler_ServeHTTP_OverviewRunsBeforeReview(t *testing.T) {
	reviewUC := &mockReviewUseCase{requestCh: make(chan usecase.ReviewRequest, 1)}
	overviewUC := &mockOverviewUseCase{
		requestCh: make(chan usecase.OverviewRequest, 1),
		proceedCh: make(chan struct{}),
	}
	queue := jobqueue.NewManager(2)
	handler := NewHandler(
		newReviewBuilder(reviewUC),
		newOverviewBuilder(overviewUC),
		nil,
		nil,
		mockInstallationTokenProvider{token: "token-1"},
		nil,
		&mockCodeEnvironmentFactory{},
		&mockRecipeLoader{},
		nil,
		testWebhookSecret,
		"peerbot",
		true,
		defaultTestReviewEvents,
		true,
		true,
		defaultTestOverviewEvents,
		true,
		false,
		defaultTestAutogenEvents,
		false,
		false,
		true,
		defaultTestReplyEvents,
		defaultTestReplyActions,
		queue,
	)
	payload := `{
		"action":"opened",
		"installation":{"id":123},
		"repository":{"full_name":"org/repo","clone_url":"https://github.com/org/repo.git"},
		"pull_request":{
			"number": 7,
			"title":"Improve API",
			"body":"details",
			"base":{"ref":"main"},
			"head":{"ref":"feature"}
		}
	}`
	req := signedRequest(t, payload, "pull_request", testWebhookSecret)
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	require.Equal(t, http.StatusAccepted, resp.Code)
	select {
	case <-overviewUC.requestCh:
	case <-time.After(time.Second):
		t.Fatal("expected overview usecase execution")
	}

	select {
	case <-reviewUC.requestCh:
		t.Fatal("expected review to wait for overview completion")
	case <-time.After(200 * time.Millisecond):
	}

	close(overviewUC.proceedCh)

	select {
	case <-reviewUC.requestCh:
	case <-time.After(time.Second):
		t.Fatal("expected review usecase execution")
	}
}

func TestHandler_ServeHTTP_ConfigDisablesReviewSkipsUsecase(t *testing.T) {
	uc := &mockReviewUseCase{requestCh: make(chan usecase.ReviewRequest, 1)}
	loader := &mockRecipeConfigLoader{
		recipe: domain.CustomRecipe{ReviewEnabled: boolPointer(false)},
	}
	handler := newTestHandler(newReviewBuilder(uc), nil, nil, nil, mockInstallationTokenProvider{token: "token-1"}, loader, nil, testWebhookSecret, "peerbot", false, true)
	payload := `{
		"action":"opened",
		"installation":{"id":123},
		"repository":{"full_name":"org/repo","clone_url":"https://github.com/org/repo.git"},
		"pull_request":{
			"number": 7,
			"title":"Improve API",
			"body":"Fixes #12",
			"base":{"ref":"main"},
			"head":{"ref":"feature"}
		}
	}`
	req := signedRequest(t, payload, "pull_request", testWebhookSecret)
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	require.Equal(t, http.StatusAccepted, resp.Code)
	select {
	case <-uc.requestCh:
		t.Fatal("expected review usecase to be skipped")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestHandler_ServeHTTP_ConfigDisablesIssueAlignmentSkipsCandidates(t *testing.T) {
	overviewUC := &mockOverviewUseCase{requestCh: make(chan usecase.OverviewRequest, 1)}
	loader := &mockRecipeConfigLoader{
		recipe: domain.CustomRecipe{
			ReviewEnabled:                 boolPointer(false),
			OverviewIssueAlignmentEnabled: boolPointer(false),
		},
	}
	tokenProvider := &issueAlignmentTokenProvider{token: "token-1"}
	handler := newTestHandler(nil, newOverviewBuilder(overviewUC), nil, nil, tokenProvider, loader, nil, testWebhookSecret, "peerbot", true, true)
	payload := `{
		"action":"opened",
		"installation":{"id":123},
		"repository":{"full_name":"org/repo","clone_url":"https://github.com/org/repo.git"},
		"pull_request":{
			"number": 7,
			"title":"Improve API",
			"body":"Fixes #12",
			"base":{"ref":"main"},
			"head":{"ref":"feature"}
		}
	}`
	req := signedRequest(t, payload, "pull_request", testWebhookSecret)
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	require.Equal(t, http.StatusAccepted, resp.Code)
	select {
	case overviewRequest := <-overviewUC.requestCh:
		require.Empty(t, overviewRequest.IssueAlignment.Candidates)
	case <-time.After(time.Second):
		t.Fatal("expected overview usecase execution")
	}
	require.Equal(t, 0, tokenProvider.issueCalls)
	require.Equal(t, 0, tokenProvider.commentCalls)
}

func TestHandler_ServeHTTP_SynchronizeActionTriggersReview(t *testing.T) {
	uc := &mockReviewUseCase{requestCh: make(chan usecase.ReviewRequest, 1)}
	handler := newTestHandler(newReviewBuilder(uc), nil, nil, nil, mockInstallationTokenProvider{token: "token-1"}, nil, nil, testWebhookSecret, "peerbot", true, true)
	payload := `{
		"action":"synchronize",
		"installation":{"id":321},
		"repository":{"full_name":"org/repo","clone_url":"https://github.com/org/repo.git"},
		"pull_request":{
			"number": 7,
			"title":"Improve API",
			"body":"details",
			"base":{"ref":"main"},
			"head":{"ref":"feature"}
		}
	}`
	req := signedRequest(t, payload, "pull_request", testWebhookSecret)
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	require.Equal(t, http.StatusAccepted, resp.Code)
	select {
	case reviewRequest := <-uc.requestCh:
		require.Equal(t, "synchronize", reviewRequest.Input.Metadata["action"])
	case <-time.After(time.Second):
		t.Fatal("expected review usecase execution")
	}
}

func TestHandler_ServeHTTP_OpenedActionDisablesOverviewWhenHandlerToggleIsFalse(t *testing.T) {
	reviewUC := &mockReviewUseCase{requestCh: make(chan usecase.ReviewRequest, 1)}
	overviewUC := &mockOverviewUseCase{requestCh: make(chan usecase.OverviewRequest, 1)}
	handler := newTestHandler(newReviewBuilder(reviewUC), newOverviewBuilder(overviewUC), nil, nil, mockInstallationTokenProvider{token: "token-1"}, nil, nil, testWebhookSecret, "peerbot", false, true)
	payload := `{
		"action":"opened",
		"installation":{"id":321},
		"repository":{"full_name":"org/repo","clone_url":"https://github.com/org/repo.git"},
		"pull_request":{
			"number": 7,
			"title":"Improve API",
			"body":"details",
			"base":{"ref":"main"},
			"head":{"ref":"feature"}
		}
	}`
	req := signedRequest(t, payload, "pull_request", testWebhookSecret)
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	require.Equal(t, http.StatusAccepted, resp.Code)
	select {
	case <-reviewUC.requestCh:
	case <-time.After(time.Second):
		t.Fatal("expected review usecase execution")
	}
	select {
	case <-overviewUC.requestCh:
		t.Fatal("expected overview usecase to be skipped")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestHandler_ServeHTTP_DisablesSuggestionsWhenHandlerToggleIsFalse(t *testing.T) {
	uc := &mockReviewUseCase{requestCh: make(chan usecase.ReviewRequest, 1)}
	handler := newTestHandler(newReviewBuilder(uc), nil, nil, nil, mockInstallationTokenProvider{token: "token-1"}, nil, nil, testWebhookSecret, "peerbot", true, false)
	payload := `{
		"action":"opened",
		"installation":{"id":321},
		"repository":{"full_name":"org/repo","clone_url":"https://github.com/org/repo.git"},
		"pull_request":{
			"number": 7,
			"title":"Improve API",
			"body":"details",
			"base":{"ref":"main"},
			"head":{"ref":"feature"}
		}
	}`
	req := signedRequest(t, payload, "pull_request", testWebhookSecret)
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	require.Equal(t, http.StatusAccepted, resp.Code)
	select {
	case reviewRequest := <-uc.requestCh:
		require.False(t, reviewRequest.Suggestions)
	case <-time.After(time.Second):
		t.Fatal("expected review usecase execution")
	}
}

func TestHandler_ServeHTTP_UnsupportedActionIsIgnored(t *testing.T) {
	uc := &mockReviewUseCase{requestCh: make(chan usecase.ReviewRequest, 1)}
	handler := newTestHandler(newReviewBuilder(uc), nil, nil, nil, mockInstallationTokenProvider{token: "token-1"}, nil, nil, testWebhookSecret, "peerbot", true, true)
	payload := `{
		"action":"edited",
		"installation":{"id":321},
		"repository":{"full_name":"org/repo","clone_url":"https://github.com/org/repo.git"},
		"pull_request":{
			"number": 7,
			"title":"Improve API",
			"body":"details",
			"base":{"ref":"main"},
			"head":{"ref":"feature"}
		}
	}`
	req := signedRequest(t, payload, "pull_request", testWebhookSecret)
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	require.Equal(t, http.StatusAccepted, resp.Code)
	require.Len(t, uc.requestCh, 0)
}

func TestHandler_ServeHTTP_ConfigReviewEventsSkipsAction(t *testing.T) {
	uc := &mockReviewUseCase{requestCh: make(chan usecase.ReviewRequest, 1)}
	loader := &mockRecipeConfigLoader{
		recipe: domain.CustomRecipe{ReviewEvents: []string{"opened"}},
	}
	handler := newTestHandler(newReviewBuilder(uc), nil, nil, nil, mockInstallationTokenProvider{token: "token-1"}, loader, nil, testWebhookSecret, "peerbot", false, true)
	payload := `{
		"action":"synchronize",
		"installation":{"id":321},
		"repository":{"full_name":"org/repo","clone_url":"https://github.com/org/repo.git"},
		"pull_request":{
			"number": 7,
			"title":"Improve API",
			"body":"details",
			"base":{"ref":"main"},
			"head":{"ref":"feature"}
		}
	}`
	req := signedRequest(t, payload, "pull_request", testWebhookSecret)
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	require.Equal(t, http.StatusAccepted, resp.Code)
	require.Len(t, uc.requestCh, 0)
}

func TestHandler_ServeHTTP_AutogenEnabledEnqueuesAutogen(t *testing.T) {
	autogenUC := &mockAutogenUseCase{requestCh: make(chan usecase.AutogenRequest, 1)}
	loader := &mockRecipeConfigLoader{
		recipe: domain.CustomRecipe{
			ReviewEnabled:   boolPointer(false),
			OverviewEnabled: boolPointer(false),
			AutogenEnabled:  boolPointer(true),
			AutogenEvents:   []string{"opened"},
			AutogenDocs:     boolPointer(true),
			AutogenTests:    boolPointer(false),
		},
	}
	handler := newTestHandler(nil, nil, newAutogenBuilder(autogenUC), nil, mockInstallationTokenProvider{token: "token-1"}, loader, nil, testWebhookSecret, "peerbot", false, true)
	payload := `{
		"action":"opened",
		"installation":{"id":321},
		"repository":{"full_name":"org/repo","clone_url":"https://github.com/org/repo.git"},
		"pull_request":{
			"number": 7,
			"title":"Improve API",
			"body":"details",
			"base":{"ref":"main"},
			"head":{"ref":"feature"}
		}
	}`
	req := signedRequest(t, payload, "pull_request", testWebhookSecret)
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	require.Equal(t, http.StatusAccepted, resp.Code)
	select {
	case autogenRequest := <-autogenUC.requestCh:
		require.True(t, autogenRequest.Docs)
		require.False(t, autogenRequest.Tests)
		require.True(t, autogenRequest.Publish)
		require.Equal(t, "feature", autogenRequest.HeadBranch)
		require.Equal(t, "opened", autogenRequest.Input.Metadata["action"])
	case <-time.After(time.Second):
		t.Fatal("expected autogen usecase execution")
	}
}

func TestHandler_ServeHTTP_AutogenDisabledWithoutDocsOrTests(t *testing.T) {
	autogenUC := &mockAutogenUseCase{requestCh: make(chan usecase.AutogenRequest, 1)}
	loader := &mockRecipeConfigLoader{
		recipe: domain.CustomRecipe{
			ReviewEnabled:   boolPointer(false),
			OverviewEnabled: boolPointer(false),
			AutogenEnabled:  boolPointer(true),
			AutogenEvents:   []string{"opened"},
			AutogenDocs:     boolPointer(false),
			AutogenTests:    boolPointer(false),
		},
	}
	handler := newTestHandler(nil, nil, newAutogenBuilder(autogenUC), nil, mockInstallationTokenProvider{token: "token-1"}, loader, nil, testWebhookSecret, "peerbot", false, true)
	payload := `{
		"action":"opened",
		"installation":{"id":321},
		"repository":{"full_name":"org/repo","clone_url":"https://github.com/org/repo.git"},
		"pull_request":{
			"number": 7,
			"title":"Improve API",
			"body":"details",
			"base":{"ref":"main"},
			"head":{"ref":"feature"}
		}
	}`
	req := signedRequest(t, payload, "pull_request", testWebhookSecret)
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	require.Equal(t, http.StatusAccepted, resp.Code)
	require.Len(t, autogenUC.requestCh, 0)
}

func TestHandler_ServeHTTP_MissingSignatureReturnsUnauthorized(t *testing.T) {
	uc := &mockReviewUseCase{requestCh: make(chan usecase.ReviewRequest, 1)}
	handler := newTestHandler(newReviewBuilder(uc), nil, nil, nil, mockInstallationTokenProvider{token: "token-1"}, nil, nil, testWebhookSecret, "peerbot", false, true)
	req := httptest.NewRequest(http.MethodPost, "/github/webhook", strings.NewReader(`{}`))
	req.Header.Set("X-GitHub-Event", "pull_request")
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	require.Equal(t, http.StatusUnauthorized, resp.Code)
	require.Len(t, uc.requestCh, 0)
}

func TestHandler_ServeHTTP_InvalidSignatureReturnsUnauthorized(t *testing.T) {
	uc := &mockReviewUseCase{requestCh: make(chan usecase.ReviewRequest, 1)}
	handler := newTestHandler(newReviewBuilder(uc), nil, nil, nil, mockInstallationTokenProvider{token: "token-1"}, nil, nil, testWebhookSecret, "peerbot", false, true)
	req := httptest.NewRequest(http.MethodPost, "/github/webhook", strings.NewReader(`{}`))
	req.Header.Set("X-GitHub-Event", "pull_request")
	req.Header.Set("X-Hub-Signature-256", "sha256=deadbeef")
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	require.Equal(t, http.StatusUnauthorized, resp.Code)
	require.Len(t, uc.requestCh, 0)
}

func TestHandler_ServeHTTP_InvalidJSONReturnsBadRequest(t *testing.T) {
	uc := &mockReviewUseCase{requestCh: make(chan usecase.ReviewRequest, 1)}
	handler := newTestHandler(newReviewBuilder(uc), nil, nil, nil, mockInstallationTokenProvider{token: "token-1"}, nil, nil, testWebhookSecret, "peerbot", false, true)
	req := signedRequest(t, `{`, "pull_request", testWebhookSecret)
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	require.Equal(t, http.StatusBadRequest, resp.Code)
	require.Len(t, uc.requestCh, 0)
}

func TestHandler_ServeHTTP_MissingRequiredFieldsReturnsBadRequest(t *testing.T) {
	uc := &mockReviewUseCase{requestCh: make(chan usecase.ReviewRequest, 1)}
	handler := newTestHandler(newReviewBuilder(uc), nil, nil, nil, mockInstallationTokenProvider{token: "token-1"}, nil, nil, testWebhookSecret, "peerbot", false, true)
	payload := `{
		"action":"opened",
		"installation":{"id":123},
		"repository":{"full_name":" "},
		"pull_request":{
			"number": 0,
			"title":"Improve API",
			"body":"details",
			"base":{"ref":""},
			"head":{"ref":" "}
		}
	}`
	req := signedRequest(t, payload, "pull_request", testWebhookSecret)
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	require.Equal(t, http.StatusBadRequest, resp.Code)
	require.Len(t, uc.requestCh, 0)
}

func TestHandler_ServeHTTP_MissingInstallationIDReturnsBadRequest(t *testing.T) {
	uc := &mockReviewUseCase{requestCh: make(chan usecase.ReviewRequest, 1)}
	logger := &spyLogger{}
	handler := newTestHandler(newReviewBuilder(uc), nil, nil, nil, mockInstallationTokenProvider{token: "token-1"}, nil, logger, testWebhookSecret, "peerbot", false, true)
	payload := `{
		"action":"opened",
		"repository":{"full_name":"org/repo","clone_url":"https://github.com/org/repo.git"},
		"pull_request":{
			"number": 7,
			"title":"Improve API",
			"body":"details",
			"base":{"ref":"main"},
			"head":{"ref":"feature"}
		}
	}`
	req := signedRequest(t, payload, "pull_request", testWebhookSecret)
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	require.Equal(t, http.StatusBadRequest, resp.Code)
	require.Len(t, uc.requestCh, 0)
	require.Eventually(t, func() bool {
		return containsEvent(logger.events, "error:GitHub webhook payload is missing installation id.")
	}, time.Second, 10*time.Millisecond)
}

func TestHandler_ServeHTTP_NonPullRequestEventIsIgnored(t *testing.T) {
	uc := &mockReviewUseCase{requestCh: make(chan usecase.ReviewRequest, 1)}
	handler := newTestHandler(newReviewBuilder(uc), nil, nil, nil, mockInstallationTokenProvider{token: "token-1"}, nil, nil, testWebhookSecret, "peerbot", false, true)
	req := signedRequest(t, `{"action":"opened"}`, "issues", testWebhookSecret)
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	require.Equal(t, http.StatusAccepted, resp.Code)
	require.Len(t, uc.requestCh, 0)
}

func TestHandler_ServeHTTP_ConfigDisablesReplyCommentSkipsUsecase(t *testing.T) {
	replyUseCase := &mockReplyCommentUseCase{requestCh: make(chan usecase.ReplyCommentRequest, 1)}
	replyBuilder := func(_ string) (usecase.ReplyCommentUseCase, error) {
		return replyUseCase, nil
	}
	prInfo := domain.ChangeRequestInfo{
		Repository:  "org/repo",
		Number:      7,
		Title:       "Title",
		Description: "Body",
		BaseRef:     "main",
		HeadRef:     "feature",
	}
	loader := &mockRecipeConfigLoader{
		recipe: domain.CustomRecipe{ReplyCommentEnabled: boolPointer(false)},
	}
	handler := newTestHandler(nil, nil, nil, replyBuilder, &replyCommentTokenProvider{token: "token-1", prInfo: prInfo}, loader, nil, testWebhookSecret, "peerbot", true, true)
	payload := `{
		"action":"created",
		"installation":{"id":123},
		"repository":{"full_name":"org/repo","clone_url":"https://github.com/org/repo.git"},
		"issue":{
			"number": 7,
			"pull_request": {}
		},
		"comment":{
			"id": 55,
			"body":"@peerbot please help",
			"user":{"login":"dev","type":"User"}
		}
	}`
	req := signedRequest(t, payload, "issue_comment", testWebhookSecret)
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	require.Equal(t, http.StatusAccepted, resp.Code)
	select {
	case <-replyUseCase.requestCh:
		t.Fatal("expected replycomment usecase to be skipped")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestHandler_ServeHTTP_ConfigReplyCommentEventsSkipsIssueComment(t *testing.T) {
	replyUseCase := &mockReplyCommentUseCase{requestCh: make(chan usecase.ReplyCommentRequest, 1)}
	replyBuilder := func(_ string) (usecase.ReplyCommentUseCase, error) {
		return replyUseCase, nil
	}
	prInfo := domain.ChangeRequestInfo{
		Repository:  "org/repo",
		Number:      7,
		Title:       "Title",
		Description: "Body",
		BaseRef:     "main",
		HeadRef:     "feature",
	}
	loader := &mockRecipeConfigLoader{
		recipe: domain.CustomRecipe{
			ReplyCommentEvents:  []string{"pull_request_review_comment"},
			ReplyCommentActions: []string{"created"},
		},
	}
	handler := newTestHandler(nil, nil, nil, replyBuilder, &replyCommentTokenProvider{token: "token-1", prInfo: prInfo}, loader, nil, testWebhookSecret, "peerbot", true, true)
	payload := `{
		"action":"created",
		"installation":{"id":123},
		"repository":{"full_name":"org/repo","clone_url":"https://github.com/org/repo.git"},
		"issue":{
			"number": 7,
			"pull_request": {}
		},
		"comment":{
			"id": 55,
			"body":"@peerbot please help",
			"user":{"login":"dev","type":"User"}
		}
	}`
	req := signedRequest(t, payload, "issue_comment", testWebhookSecret)
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	require.Equal(t, http.StatusAccepted, resp.Code)
	select {
	case <-replyUseCase.requestCh:
		t.Fatal("expected replycomment usecase to be skipped")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestHandler_ServeHTTP_ResponseDoesNotWaitForUsecase(t *testing.T) {
	uc := &mockReviewUseCase{
		requestCh: make(chan usecase.ReviewRequest, 1),
		proceedCh: make(chan struct{}),
	}
	handler := newTestHandler(newReviewBuilder(uc), nil, nil, nil, mockInstallationTokenProvider{token: "token-1"}, nil, nil, testWebhookSecret, "peerbot", false, true)
	payload := `{
		"action":"opened",
		"installation":{"id":123},
		"repository":{"full_name":"org/repo","clone_url":"https://github.com/org/repo.git"},
		"pull_request":{
			"number": 7,
			"title":"Improve API",
			"body":"details",
			"base":{"ref":"main"},
			"head":{"ref":"feature"}
		}
	}`
	req := signedRequest(t, payload, "pull_request", testWebhookSecret)
	resp := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		handler.ServeHTTP(resp, req)
		close(done)
	}()

	select {
	case <-done:
		require.Equal(t, http.StatusAccepted, resp.Code)
	case <-time.After(200 * time.Millisecond):
		t.Fatal("handler did not return immediately")
	}

	select {
	case <-uc.requestCh:
	case <-time.After(time.Second):
		t.Fatal("expected review usecase execution")
	}
	close(uc.proceedCh)
}

func TestHandler_ServeHTTP_UsecaseErrorStillReturnsAccepted(t *testing.T) {
	uc := &mockReviewUseCase{
		requestCh: make(chan usecase.ReviewRequest, 1),
		err:       errors.New("review failed"),
	}
	logger := &spyLogger{}
	handler := newTestHandler(newReviewBuilder(uc), nil, nil, nil, mockInstallationTokenProvider{token: "token-1"}, nil, logger, testWebhookSecret, "peerbot", false, true)
	payload := `{
		"action":"opened",
		"installation":{"id":123},
		"repository":{"full_name":"org/repo","clone_url":"https://github.com/org/repo.git"},
		"pull_request":{
			"number": 7,
			"title":"Improve API",
			"body":"details",
			"base":{"ref":"main"},
			"head":{"ref":"feature"}
		}
	}`
	req := signedRequest(t, payload, "pull_request", testWebhookSecret)
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	require.Equal(t, http.StatusAccepted, resp.Code)
	select {
	case <-uc.requestCh:
	case <-time.After(time.Second):
		t.Fatal("expected review usecase execution")
	}
	require.Eventually(t, func() bool {
		return containsEvent(logger.events, "debug:GitHub webhook review failed for")
	}, time.Second, 10*time.Millisecond)
}

func TestHandler_ServeHTTP_UsecasePanicStillReturnsAccepted(t *testing.T) {
	uc := &mockReviewUseCase{
		requestCh: make(chan usecase.ReviewRequest, 1),
		panicVal:  "boom",
	}
	logger := &spyLogger{}
	handler := newTestHandler(newReviewBuilder(uc), nil, nil, nil, mockInstallationTokenProvider{token: "token-1"}, nil, logger, testWebhookSecret, "peerbot", false, true)
	payload := `{
		"action":"opened",
		"installation":{"id":123},
		"repository":{"full_name":"org/repo","clone_url":"https://github.com/org/repo.git"},
		"pull_request":{
			"number": 7,
			"title":"Improve API",
			"body":"details",
			"base":{"ref":"main"},
			"head":{"ref":"feature"}
		}
	}`
	req := signedRequest(t, payload, "pull_request", testWebhookSecret)
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	require.Equal(t, http.StatusAccepted, resp.Code)
	require.Eventually(t, func() bool {
		return containsEvent(logger.events, "error:GitHub webhook review panicked for")
	}, time.Second, 10*time.Millisecond)
}

func TestHandler_ServeHTTP_TokenResolutionFailureReturnsBadGateway(t *testing.T) {
	uc := &mockReviewUseCase{requestCh: make(chan usecase.ReviewRequest, 1)}
	handler := newTestHandler(newReviewBuilder(uc), nil, nil, nil, mockInstallationTokenProvider{err: errors.New("boom")}, nil, nil, testWebhookSecret, "peerbot", false, true)
	payload := `{
		"action":"opened",
		"installation":{"id":123},
		"repository":{"full_name":"org/repo","clone_url":"https://github.com/org/repo.git"},
		"pull_request":{
			"number": 7,
			"title":"Improve API",
			"body":"details",
			"base":{"ref":"main"},
			"head":{"ref":"feature"}
		}
	}`
	req := signedRequest(t, payload, "pull_request", testWebhookSecret)
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	require.Equal(t, http.StatusBadGateway, resp.Code)
	require.Len(t, uc.requestCh, 0)
}
