package github

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	githubvcs "bentos-backend/adapter/outbound/vcs/github"
	"bentos-backend/domain"
	"bentos-backend/usecase"
	"github.com/stretchr/testify/require"
)

type fakeClient struct {
	reviewInputs []githubvcs.CreateReviewCommentInput
	bodies       []string
	reviewErr    error
	commentErr   error
}

func (f *fakeClient) CreateComment(_ context.Context, _ string, _ int, body string) error {
	if f.commentErr != nil {
		return f.commentErr
	}
	f.bodies = append(f.bodies, body)
	return nil
}

func (f *fakeClient) CreateReviewComment(_ context.Context, _ string, _ int, input githubvcs.CreateReviewCommentInput) error {
	if f.reviewErr != nil {
		return f.reviewErr
	}
	f.reviewInputs = append(f.reviewInputs, input)
	return nil
}

type spyLogger struct {
	events []string
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

func TestPublisher_PublishPostsAnchoredFindingsAndSummary(t *testing.T) {
	client := &fakeClient{}
	pub := NewPublisher(client, nil)

	err := pub.Publish(context.Background(), usecase.ReviewPublishResult{
		Target: domain.ReviewTarget{
			Repository:          "org/repo",
			ChangeRequestNumber: 11,
		},
		Findings: []domain.Finding{
			{
				FilePath:   "a.go",
				StartLine:  7,
				EndLine:    7,
				Severity:   domain.FindingSeverityMajor,
				Title:      "Nil risk",
				Detail:     "Potential nil dereference.",
				Suggestion: "Guard before use.",
			},
			{
				FilePath:   "b.go",
				StartLine:  10,
				EndLine:    12,
				Severity:   domain.FindingSeverityMinor,
				Title:      "Complex branch",
				Detail:     "Too many nested branches.",
				Suggestion: "",
			},
		},
		Summary: "2 findings.",
	})
	require.NoError(t, err)
	require.Len(t, client.reviewInputs, 2)
	require.Equal(t, 7, client.reviewInputs[0].StartLine)
	require.Equal(t, 12, client.reviewInputs[1].EndLine)
	require.Len(t, client.bodies, 1)
	require.Contains(t, client.bodies[0], "Review summary")
	require.Contains(t, client.bodies[0], "2 findings.")
}

func TestPublisher_PublishSkipsInvalidLocalAnchor(t *testing.T) {
	client := &fakeClient{}
	logger := &spyLogger{}
	pub := NewPublisher(client, logger)

	err := pub.Publish(context.Background(), usecase.ReviewPublishResult{
		Target: domain.ReviewTarget{
			Repository:          "org/repo",
			ChangeRequestNumber: 11,
		},
		Findings: []domain.Finding{
			{
				FilePath:   "",
				StartLine:  1,
				EndLine:    1,
				Severity:   domain.FindingSeverityMajor,
				Title:      "Missing path",
				Detail:     "No path.",
				Suggestion: "",
			},
		},
		Summary: "done",
	})
	require.NoError(t, err)
	require.Empty(t, client.reviewInputs)
	require.Len(t, client.bodies, 1)
	require.True(t, containsEvent(logger.events, "warn:Skipped one GitHub review comment because its anchor is invalid."))
}

func TestPublisher_PublishSkipsInvalidAnchorFromClient(t *testing.T) {
	client := &fakeClient{
		reviewErr: &githubvcs.InvalidAnchorError{Message: "invalid review comment anchor"},
	}
	logger := &spyLogger{}
	pub := NewPublisher(client, logger)

	err := pub.Publish(context.Background(), usecase.ReviewPublishResult{
		Target: domain.ReviewTarget{
			Repository:          "org/repo",
			ChangeRequestNumber: 11,
		},
		Findings: []domain.Finding{
			{
				FilePath:   "a.go",
				StartLine:  1,
				EndLine:    1,
				Severity:   domain.FindingSeverityMajor,
				Title:      "x",
				Detail:     "y",
				Suggestion: "",
			},
		},
		Summary: "done",
	})
	require.NoError(t, err)
	require.Len(t, client.bodies, 1)
	require.True(t, containsEvent(logger.events, "warn:Skipped one GitHub review comment because its anchor is invalid."))
}

func TestPublisher_PublishFailsForNonAnchorError(t *testing.T) {
	client := &fakeClient{
		reviewErr: errors.New("network fail"),
	}
	pub := NewPublisher(client, nil)

	err := pub.Publish(context.Background(), usecase.ReviewPublishResult{
		Target: domain.ReviewTarget{
			Repository:          "org/repo",
			ChangeRequestNumber: 11,
		},
		Findings: []domain.Finding{
			{
				FilePath:   "a.go",
				StartLine:  1,
				EndLine:    1,
				Severity:   domain.FindingSeverityMajor,
				Title:      "x",
				Detail:     "y",
				Suggestion: "",
			},
		},
		Summary: "done",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "network fail")
}

func TestPublisher_PublishPostsOnlySummaryWhenNoFindings(t *testing.T) {
	client := &fakeClient{}
	pub := NewPublisher(client, nil)

	err := pub.Publish(context.Background(), usecase.ReviewPublishResult{
		Target: domain.ReviewTarget{
			Repository:          "org/repo",
			ChangeRequestNumber: 11,
		},
		Summary: "",
	})
	require.NoError(t, err)
	require.Empty(t, client.reviewInputs)
	require.Len(t, client.bodies, 1)
	require.Contains(t, client.bodies[0], "No significant review findings")
}

func containsEvent(events []string, target string) bool {
	for _, event := range events {
		if strings.Contains(event, target) {
			return true
		}
	}
	return false
}
