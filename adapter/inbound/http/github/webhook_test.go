package github

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	githubvcs "bentos-backend/adapter/outbound/vcs/github"
	"bentos-backend/usecase"
	"github.com/stretchr/testify/require"
)

const testWebhookSecret = "test-secret"

type mockUseCase struct {
	requestCh chan usecase.ChangeRequestRequest
	ctxCh     chan context.Context
	proceedCh chan struct{}
	err       error
	panicVal  any
}

func (m *mockUseCase) Execute(ctx context.Context, request usecase.ChangeRequestRequest) (usecase.ChangeRequestExecutionResult, error) {
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
	return usecase.ChangeRequestExecutionResult{}, m.err
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

func (m mockInstallationTokenProvider) GetPullRequestInfo(_ context.Context, _ string, _ int) (githubvcs.PullRequestInfo, error) {
	return githubvcs.PullRequestInfo{}, errors.New("not implemented")
}

func (m mockInstallationTokenProvider) GetPullRequestReview(_ context.Context, _ string, _ int, _ int64) (githubvcs.PullRequestReviewSummary, error) {
	return githubvcs.PullRequestReviewSummary{}, errors.New("not implemented")
}

func (m mockInstallationTokenProvider) ListIssueComments(_ context.Context, _ string, _ int) ([]githubvcs.IssueComment, error) {
	return nil, errors.New("not implemented")
}

func (m mockInstallationTokenProvider) ListReviewComments(_ context.Context, _ string, _ int) ([]githubvcs.ReviewComment, error) {
	return nil, errors.New("not implemented")
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

func TestHandler_ServeHTTP_ValidPayloadReturnsAcceptedAndMapsRequest(t *testing.T) {
	uc := &mockUseCase{
		requestCh: make(chan usecase.ChangeRequestRequest, 1),
		ctxCh:     make(chan context.Context, 1),
	}
	logger := &spyLogger{}
	handler := NewHandler(uc, nil, mockInstallationTokenProvider{token: "token-1"}, logger, testWebhookSecret, "autogitbot", true, true)
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
	case reviewRequest := <-uc.requestCh:
		require.Equal(t, "org/repo", reviewRequest.Repository)
		require.Equal(t, "https://x-access-token:token-1@github.com/org/repo.git", reviewRequest.RepoURL)
		require.Equal(t, 7, reviewRequest.ChangeRequestNumber)
		require.Equal(t, "main", reviewRequest.Base)
		require.Equal(t, "feature", reviewRequest.Head)
		require.Equal(t, "opened", reviewRequest.Metadata["action"])
		require.True(t, reviewRequest.EnableOverview)
		require.True(t, reviewRequest.EnableSuggestions)
	case <-time.After(time.Second):
		t.Fatal("expected review usecase execution")
	}

	select {
	case reviewContext := <-uc.ctxCh:
		require.Equal(t, "123", githubvcs.InstallationIDFromContext(reviewContext))
	case <-time.After(time.Second):
		t.Fatal("expected review usecase context")
	}

	require.Eventually(t, func() bool {
		return containsEvent(logger.events, `info:Pre-usecase input snapshot source="webhook" action="opened" repository="org/repo" changeRequestNumber=7 enableOverview=true enableSuggestions=true.`)
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

func TestHandler_ServeHTTP_SynchronizeActionTriggersReview(t *testing.T) {
	uc := &mockUseCase{requestCh: make(chan usecase.ChangeRequestRequest, 1)}
	handler := NewHandler(uc, nil, mockInstallationTokenProvider{token: "token-1"}, nil, testWebhookSecret, "autogitbot", true, true)
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
		require.False(t, reviewRequest.EnableOverview)
	case <-time.After(time.Second):
		t.Fatal("expected review usecase execution")
	}
}

func TestHandler_ServeHTTP_OpenedActionDisablesOverviewWhenHandlerToggleIsFalse(t *testing.T) {
	uc := &mockUseCase{requestCh: make(chan usecase.ChangeRequestRequest, 1)}
	handler := NewHandler(uc, nil, mockInstallationTokenProvider{token: "token-1"}, nil, testWebhookSecret, "autogitbot", false, true)
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
		require.False(t, reviewRequest.EnableOverview)
	case <-time.After(time.Second):
		t.Fatal("expected review usecase execution")
	}
}

func TestHandler_ServeHTTP_DisablesSuggestionsWhenHandlerToggleIsFalse(t *testing.T) {
	uc := &mockUseCase{requestCh: make(chan usecase.ChangeRequestRequest, 1)}
	handler := NewHandler(uc, nil, mockInstallationTokenProvider{token: "token-1"}, nil, testWebhookSecret, "autogitbot", true, false)
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
		require.False(t, reviewRequest.EnableSuggestions)
	case <-time.After(time.Second):
		t.Fatal("expected review usecase execution")
	}
}

func TestHandler_ServeHTTP_UnsupportedActionIsIgnored(t *testing.T) {
	uc := &mockUseCase{requestCh: make(chan usecase.ChangeRequestRequest, 1)}
	handler := NewHandler(uc, nil, mockInstallationTokenProvider{token: "token-1"}, nil, testWebhookSecret, "autogitbot", true, true)
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

func TestHandler_ServeHTTP_MissingSignatureReturnsUnauthorized(t *testing.T) {
	uc := &mockUseCase{requestCh: make(chan usecase.ChangeRequestRequest, 1)}
	handler := NewHandler(uc, nil, mockInstallationTokenProvider{token: "token-1"}, nil, testWebhookSecret, "autogitbot", true, true)
	req := httptest.NewRequest(http.MethodPost, "/github/webhook", strings.NewReader(`{}`))
	req.Header.Set("X-GitHub-Event", "pull_request")
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	require.Equal(t, http.StatusUnauthorized, resp.Code)
	require.Len(t, uc.requestCh, 0)
}

func TestHandler_ServeHTTP_InvalidSignatureReturnsUnauthorized(t *testing.T) {
	uc := &mockUseCase{requestCh: make(chan usecase.ChangeRequestRequest, 1)}
	handler := NewHandler(uc, nil, mockInstallationTokenProvider{token: "token-1"}, nil, testWebhookSecret, "autogitbot", true, true)
	req := httptest.NewRequest(http.MethodPost, "/github/webhook", strings.NewReader(`{}`))
	req.Header.Set("X-GitHub-Event", "pull_request")
	req.Header.Set("X-Hub-Signature-256", "sha256=deadbeef")
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	require.Equal(t, http.StatusUnauthorized, resp.Code)
	require.Len(t, uc.requestCh, 0)
}

func TestHandler_ServeHTTP_InvalidJSONReturnsBadRequest(t *testing.T) {
	uc := &mockUseCase{requestCh: make(chan usecase.ChangeRequestRequest, 1)}
	handler := NewHandler(uc, nil, mockInstallationTokenProvider{token: "token-1"}, nil, testWebhookSecret, "autogitbot", true, true)
	req := signedRequest(t, `{`, "pull_request", testWebhookSecret)
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	require.Equal(t, http.StatusBadRequest, resp.Code)
	require.Len(t, uc.requestCh, 0)
}

func TestHandler_ServeHTTP_MissingRequiredFieldsReturnsBadRequest(t *testing.T) {
	uc := &mockUseCase{requestCh: make(chan usecase.ChangeRequestRequest, 1)}
	handler := NewHandler(uc, nil, mockInstallationTokenProvider{token: "token-1"}, nil, testWebhookSecret, "autogitbot", true, true)
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
	uc := &mockUseCase{requestCh: make(chan usecase.ChangeRequestRequest, 1)}
	logger := &spyLogger{}
	handler := NewHandler(uc, nil, mockInstallationTokenProvider{token: "token-1"}, logger, testWebhookSecret, "autogitbot", true, true)
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
	uc := &mockUseCase{requestCh: make(chan usecase.ChangeRequestRequest, 1)}
	handler := NewHandler(uc, nil, mockInstallationTokenProvider{token: "token-1"}, nil, testWebhookSecret, "autogitbot", true, true)
	req := signedRequest(t, `{"action":"opened"}`, "issues", testWebhookSecret)
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	require.Equal(t, http.StatusAccepted, resp.Code)
	require.Len(t, uc.requestCh, 0)
}

func TestHandler_ServeHTTP_ResponseDoesNotWaitForUsecase(t *testing.T) {
	uc := &mockUseCase{
		requestCh: make(chan usecase.ChangeRequestRequest, 1),
		proceedCh: make(chan struct{}),
	}
	handler := NewHandler(uc, nil, mockInstallationTokenProvider{token: "token-1"}, nil, testWebhookSecret, "autogitbot", true, true)
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
	uc := &mockUseCase{
		requestCh: make(chan usecase.ChangeRequestRequest, 1),
		err:       errors.New("review failed"),
	}
	logger := &spyLogger{}
	handler := NewHandler(uc, nil, mockInstallationTokenProvider{token: "token-1"}, logger, testWebhookSecret, "autogitbot", true, true)
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
		return containsEvent(logger.events, "error:GitHub webhook background review failed.")
	}, time.Second, 10*time.Millisecond)
}

func TestHandler_ServeHTTP_UsecasePanicStillReturnsAccepted(t *testing.T) {
	uc := &mockUseCase{
		requestCh: make(chan usecase.ChangeRequestRequest, 1),
		panicVal:  "boom",
	}
	logger := &spyLogger{}
	handler := NewHandler(uc, nil, mockInstallationTokenProvider{token: "token-1"}, logger, testWebhookSecret, "autogitbot", true, true)
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
		return containsEvent(logger.events, "error:GitHub webhook background review panicked.")
	}, time.Second, 10*time.Millisecond)
}

func TestHandler_ServeHTTP_TokenResolutionFailureReturnsBadGateway(t *testing.T) {
	uc := &mockUseCase{requestCh: make(chan usecase.ChangeRequestRequest, 1)}
	handler := NewHandler(uc, nil, mockInstallationTokenProvider{err: errors.New("boom")}, nil, testWebhookSecret, "autogitbot", true, true)
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

func signedRequest(t *testing.T, payload string, event string, secret string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/github/webhook", strings.NewReader(payload))
	req.Header.Set("X-GitHub-Event", event)
	req.Header.Set("X-Hub-Signature-256", signPayload(payload, secret))
	return req
}

func signPayload(payload string, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(payload))
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func containsEvent(events []string, target string) bool {
	for _, event := range events {
		if strings.Contains(event, target) {
			return true
		}
	}
	return false
}
