package github

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"bentos-backend/usecase"
	"github.com/stretchr/testify/require"
)

type mockUseCase struct {
	requestCh chan usecase.ReviewRequest
	proceedCh chan struct{}
	err       error
}

func (m *mockUseCase) Execute(_ context.Context, request usecase.ReviewRequest) (usecase.ReviewExecutionResult, error) {
	if m.requestCh != nil {
		m.requestCh <- request
	}
	if m.proceedCh != nil {
		<-m.proceedCh
	}
	return usecase.ReviewExecutionResult{}, m.err
}

func TestHandler_ServeHTTP_ValidPayloadReturnsAcceptedAndMapsRequest(t *testing.T) {
	uc := &mockUseCase{requestCh: make(chan usecase.ReviewRequest, 1)}
	handler := NewHandler(uc)
	payload := `{
		"action":"opened",
		"repository":{"full_name":"org/repo"},
		"pull_request":{
			"number": 7,
			"title":"Improve API",
			"body":"details",
			"base":{"ref":"main"},
			"head":{"ref":"feature"}
		}
	}`
	req := httptest.NewRequest(http.MethodPost, "/github/webhook", strings.NewReader(payload))
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	require.Equal(t, http.StatusAccepted, resp.Code)

	select {
	case reviewRequest := <-uc.requestCh:
		require.Equal(t, "org/repo", reviewRequest.Repository)
		require.Equal(t, 7, reviewRequest.ChangeRequestNumber)
	case <-time.After(time.Second):
		t.Fatal("expected review usecase execution")
	}
}

func TestHandler_ServeHTTP_InvalidJSONReturnsBadRequest(t *testing.T) {
	uc := &mockUseCase{requestCh: make(chan usecase.ReviewRequest, 1)}
	handler := NewHandler(uc)
	req := httptest.NewRequest(http.MethodPost, "/github/webhook", strings.NewReader(`{`))
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	require.Equal(t, http.StatusBadRequest, resp.Code)
	require.Len(t, uc.requestCh, 0)
}

func TestHandler_ServeHTTP_MissingRequiredFieldsReturnsBadRequest(t *testing.T) {
	uc := &mockUseCase{requestCh: make(chan usecase.ReviewRequest, 1)}
	handler := NewHandler(uc)
	payload := `{
		"action":"opened",
		"repository":{"full_name":" "},
		"pull_request":{
			"number": 0,
			"title":"Improve API",
			"body":"details",
			"base":{"ref":""},
			"head":{"ref":" "}
		}
	}`
	req := httptest.NewRequest(http.MethodPost, "/github/webhook", strings.NewReader(payload))
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	require.Equal(t, http.StatusBadRequest, resp.Code)
	require.Len(t, uc.requestCh, 0)
}

func TestHandler_ServeHTTP_ResponseDoesNotWaitForUsecase(t *testing.T) {
	uc := &mockUseCase{
		requestCh: make(chan usecase.ReviewRequest, 1),
		proceedCh: make(chan struct{}),
	}
	handler := NewHandler(uc)
	payload := `{
		"action":"opened",
		"repository":{"full_name":"org/repo"},
		"pull_request":{
			"number": 7,
			"title":"Improve API",
			"body":"details",
			"base":{"ref":"main"},
			"head":{"ref":"feature"}
		}
	}`
	req := httptest.NewRequest(http.MethodPost, "/github/webhook", strings.NewReader(payload))
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
		requestCh: make(chan usecase.ReviewRequest, 1),
		err:       errors.New("review failed"),
	}
	handler := NewHandler(uc)
	payload := `{
		"action":"opened",
		"repository":{"full_name":"org/repo"},
		"pull_request":{
			"number": 7,
			"title":"Improve API",
			"body":"details",
			"base":{"ref":"main"},
			"head":{"ref":"feature"}
		}
	}`
	req := httptest.NewRequest(http.MethodPost, "/github/webhook", strings.NewReader(payload))
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	require.Equal(t, http.StatusAccepted, resp.Code)
	select {
	case <-uc.requestCh:
	case <-time.After(time.Second):
		t.Fatal("expected review usecase execution")
	}
}
