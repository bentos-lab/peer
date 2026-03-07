package gitlab

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"bentos-backend/usecase"
	"github.com/stretchr/testify/require"
)

type mockUseCase struct {
	requestCh chan usecase.ChangeRequestRequest
	proceedCh chan struct{}
	err       error
	panicVal  any
}

func (m *mockUseCase) Execute(_ context.Context, request usecase.ChangeRequestRequest) (usecase.ChangeRequestExecutionResult, error) {
	if m.panicVal != nil {
		panic(m.panicVal)
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
	uc := &mockUseCase{requestCh: make(chan usecase.ChangeRequestRequest, 1)}
	logger := &spyLogger{}
	handler := NewHandler(uc, logger)
	payload := `{
		"object_kind":"merge_request",
		"project":{"path_with_namespace":"group/repo"},
		"object_attributes":{
			"iid": 18,
			"title":"Refactor worker",
			"description":"details",
			"target_branch":"main",
			"source_branch":"feature",
			"action":"open"
		}
	}`
	req := httptest.NewRequest(http.MethodPost, "/gitlab/webhook", strings.NewReader(payload))
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	require.Equal(t, http.StatusAccepted, resp.Code)

	select {
	case reviewRequest := <-uc.requestCh:
		require.Equal(t, "group/repo", reviewRequest.Repository)
		require.Equal(t, 18, reviewRequest.ChangeRequestNumber)
	case <-time.After(time.Second):
		t.Fatal("expected review usecase execution")
	}
}

func TestHandler_ServeHTTP_InvalidJSONReturnsBadRequest(t *testing.T) {
	uc := &mockUseCase{requestCh: make(chan usecase.ChangeRequestRequest, 1)}
	handler := NewHandler(uc, nil)
	req := httptest.NewRequest(http.MethodPost, "/gitlab/webhook", strings.NewReader(`{`))
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	require.Equal(t, http.StatusBadRequest, resp.Code)
	require.Len(t, uc.requestCh, 0)
}

func TestHandler_ServeHTTP_MissingRequiredFieldsReturnsBadRequest(t *testing.T) {
	uc := &mockUseCase{requestCh: make(chan usecase.ChangeRequestRequest, 1)}
	handler := NewHandler(uc, nil)
	payload := `{
		"object_kind":"merge_request",
		"project":{"path_with_namespace":" "},
		"object_attributes":{
			"iid": 0,
			"title":"Refactor worker",
			"description":"details",
			"target_branch":"",
			"source_branch":" ",
			"action":"open"
		}
	}`
	req := httptest.NewRequest(http.MethodPost, "/gitlab/webhook", strings.NewReader(payload))
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	require.Equal(t, http.StatusBadRequest, resp.Code)
	require.Len(t, uc.requestCh, 0)
}

func TestHandler_ServeHTTP_ResponseDoesNotWaitForUsecase(t *testing.T) {
	uc := &mockUseCase{
		requestCh: make(chan usecase.ChangeRequestRequest, 1),
		proceedCh: make(chan struct{}),
	}
	handler := NewHandler(uc, nil)
	payload := `{
		"object_kind":"merge_request",
		"project":{"path_with_namespace":"group/repo"},
		"object_attributes":{
			"iid": 18,
			"title":"Refactor worker",
			"description":"details",
			"target_branch":"main",
			"source_branch":"feature",
			"action":"open"
		}
	}`
	req := httptest.NewRequest(http.MethodPost, "/gitlab/webhook", strings.NewReader(payload))
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
	handler := NewHandler(uc, logger)
	payload := `{
		"object_kind":"merge_request",
		"project":{"path_with_namespace":"group/repo"},
		"object_attributes":{
			"iid": 18,
			"title":"Refactor worker",
			"description":"details",
			"target_branch":"main",
			"source_branch":"feature",
			"action":"open"
		}
	}`
	req := httptest.NewRequest(http.MethodPost, "/gitlab/webhook", strings.NewReader(payload))
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	require.Equal(t, http.StatusAccepted, resp.Code)
	select {
	case <-uc.requestCh:
	case <-time.After(time.Second):
		t.Fatal("expected review usecase execution")
	}
	require.Eventually(t, func() bool {
		return containsEvent(logger.events, "error:GitLab webhook background review failed.")
	}, time.Second, 10*time.Millisecond)
}

func TestHandler_ServeHTTP_UsecasePanicStillReturnsAccepted(t *testing.T) {
	uc := &mockUseCase{
		requestCh: make(chan usecase.ChangeRequestRequest, 1),
		panicVal:  "boom",
	}
	logger := &spyLogger{}
	handler := NewHandler(uc, logger)
	payload := `{
		"object_kind":"merge_request",
		"project":{"path_with_namespace":"group/repo"},
		"object_attributes":{
			"iid": 18,
			"title":"Refactor worker",
			"description":"details",
			"target_branch":"main",
			"source_branch":"feature",
			"action":"open"
		}
	}`
	req := httptest.NewRequest(http.MethodPost, "/gitlab/webhook", strings.NewReader(payload))
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	require.Equal(t, http.StatusAccepted, resp.Code)
	require.Eventually(t, func() bool {
		return containsEvent(logger.events, "error:GitLab webhook background review panicked.")
	}, time.Second, 10*time.Millisecond)
}

func containsEvent(events []string, target string) bool {
	for _, event := range events {
		if strings.Contains(event, target) {
			return true
		}
	}
	return false
}
