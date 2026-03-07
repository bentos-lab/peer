package gitlab

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"bentos-backend/domain"
	"bentos-backend/usecase"
	"github.com/stretchr/testify/require"
)

type fakeClient struct {
	bodies []string
	err    error
}

func (f *fakeClient) CreateMergeRequestNote(_ context.Context, _ string, _ int, body string) error {
	if f.err != nil {
		return f.err
	}
	f.bodies = append(f.bodies, body)
	return nil
}

type spyLogger struct {
	events []string
}

func (s *spyLogger) Tracef(format string, args ...any) {
	s.events = append(s.events, "trace:"+fmt.Sprintf(format, args...))
}

func (s *spyLogger) Debugf(format string, args ...any) {
	s.events = append(s.events, "debug:"+fmt.Sprintf(format, args...))
}

func (s *spyLogger) Infof(format string, args ...any) {
	s.events = append(s.events, "info:"+fmt.Sprintf(format, args...))
}

func (s *spyLogger) Warnf(format string, args ...any) {
	s.events = append(s.events, "warn:"+fmt.Sprintf(format, args...))
}

func (s *spyLogger) Errorf(format string, args ...any) {
	s.events = append(s.events, "error:"+fmt.Sprintf(format, args...))
}

func containsEvent(events []string, target string) bool {
	for _, event := range events {
		if strings.Contains(event, target) {
			return true
		}
	}
	return false
}

func TestPublisher_Publish(t *testing.T) {
	client := &fakeClient{}
	logger := &spyLogger{}
	pub := NewPublisher(client, logger)
	err := pub.Publish(context.Background(), usecase.ReviewPublishResult{
		Target: domain.ReviewTarget{
			Repository:          "group/repo",
			ChangeRequestNumber: 22,
		},
		Messages: []domain.ReviewMessage{{Title: "A", Body: "B"}},
	})
	require.NoError(t, err)
	require.Len(t, client.bodies, 1)
	require.Contains(t, client.bodies[0], "A")
	require.True(t, containsEvent(logger.events, "debug:GitLab review note metadata state=\"success\""))
	require.True(t, containsEvent(logger.events, "trace:GitLab review note content state=\"success\""))
}

func TestPublisher_PublishFailsWhenClientFails(t *testing.T) {
	client := &fakeClient{err: errors.New("network")}
	logger := &spyLogger{}
	pub := NewPublisher(client, logger)

	err := pub.Publish(context.Background(), usecase.ReviewPublishResult{
		Target: domain.ReviewTarget{
			Repository:          "group/repo",
			ChangeRequestNumber: 22,
		},
		Messages: []domain.ReviewMessage{{Title: "A", Body: "B"}},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "network")
	require.True(t, containsEvent(logger.events, "debug:GitLab review note metadata state=\"failed\""))
	require.True(t, containsEvent(logger.events, "trace:GitLab review note content state=\"failed\""))
}
