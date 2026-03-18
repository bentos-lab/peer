package github

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
	reviewInputs []domain.ReviewCommentInput
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

func (f *fakeClient) CreateReviewComment(_ context.Context, _ string, _ int, input domain.ReviewCommentInput) error {
	if f.reviewErr != nil {
		return f.reviewErr
	}
	f.reviewInputs = append(f.reviewInputs, input)
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

func TestPublisher_PublishPostsAnchoredFindingsAndSummary(t *testing.T) {
	client := &fakeClient{}
	logger := &spyLogger{}
	pub := NewPublisher(client, logger)

	err := pub.Publish(context.Background(), usecase.ReviewPublishResult{
		Target: domain.ChangeRequestTarget{
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
	require.True(t, containsEvent(logger.events, "debug:GitHub review comment metadata state=\"success\""))
	require.True(t, containsEvent(logger.events, "trace:GitHub review comment content state=\"success\""))
	require.True(t, containsEvent(logger.events, "trace:GitHub review summary content state=\"success\""))
}

func TestPublisher_PublishPrependsRecipeWarningToSummary(t *testing.T) {
	client := &fakeClient{}
	pub := NewPublisher(client, nil)

	err := pub.Publish(context.Background(), usecase.ReviewPublishResult{
		Target: domain.ChangeRequestTarget{
			Repository:          "org/repo",
			ChangeRequestNumber: 11,
		},
		Summary:        "summary",
		RecipeWarnings: []string{".peer/rules.md"},
	})
	require.NoError(t, err)
	require.Len(t, client.bodies, 1)
	require.True(t, strings.HasPrefix(client.bodies[0], "> [!WARNING]"))
	require.Contains(t, client.bodies[0], ".peer/rules.md")
	require.Contains(t, client.bodies[0], "Review summary")
}

func TestPublisher_PublishRendersReplaceSuggestedChangeBlock(t *testing.T) {
	client := &fakeClient{}
	pub := NewPublisher(client, nil)

	err := pub.Publish(context.Background(), usecase.ReviewPublishResult{
		Target: domain.ChangeRequestTarget{
			Repository:          "org/repo",
			ChangeRequestNumber: 11,
		},
		Findings: []domain.Finding{
			{
				FilePath:  "a.go",
				StartLine: 7,
				EndLine:   7,
				Severity:  domain.FindingSeverityMajor,
				Title:     "Nil risk",
				Detail:    "Potential nil dereference.",
				SuggestedChange: &domain.SuggestedChange{
					StartLine:   15,
					EndLine:     18,
					Kind:        domain.SuggestedChangeKindReplace,
					Replacement: "if value == nil {\n\treturn err\n}",
				},
			},
		},
		Summary: "done",
	})
	require.NoError(t, err)
	require.Len(t, client.reviewInputs, 1)
	require.Equal(t, 15, client.reviewInputs[0].StartLine)
	require.Equal(t, 18, client.reviewInputs[0].EndLine)
	require.Contains(t, client.reviewInputs[0].Body, "```suggestion")
	require.Contains(t, client.reviewInputs[0].Body, "if value == nil")
}

func TestPublisher_PublishRendersDeleteSuggestedChangeBlock(t *testing.T) {
	client := &fakeClient{}
	pub := NewPublisher(client, nil)

	err := pub.Publish(context.Background(), usecase.ReviewPublishResult{
		Target: domain.ChangeRequestTarget{
			Repository:          "org/repo",
			ChangeRequestNumber: 11,
		},
		Findings: []domain.Finding{
			{
				FilePath:  "a.go",
				StartLine: 7,
				EndLine:   7,
				Severity:  domain.FindingSeverityMajor,
				Title:     "Dead code",
				Detail:    "Remove dead branch.",
				SuggestedChange: &domain.SuggestedChange{
					StartLine:   8,
					EndLine:     8,
					Kind:        domain.SuggestedChangeKindDelete,
					Replacement: "",
				},
			},
		},
		Summary: "done",
	})
	require.NoError(t, err)
	require.Len(t, client.reviewInputs, 1)
	require.Equal(t, 8, client.reviewInputs[0].StartLine)
	require.Equal(t, 8, client.reviewInputs[0].EndLine)
	require.Contains(t, client.reviewInputs[0].Body, "```suggestion\n\n```")
}

func TestPublisher_PublishSkipsFindingWhenSuggestedChangeRangeIsInvalid(t *testing.T) {
	client := &fakeClient{}
	logger := &spyLogger{}
	pub := NewPublisher(client, logger)

	err := pub.Publish(context.Background(), usecase.ReviewPublishResult{
		Target: domain.ChangeRequestTarget{
			Repository:          "org/repo",
			ChangeRequestNumber: 11,
		},
		Findings: []domain.Finding{
			{
				FilePath:  "a.go",
				StartLine: 7,
				EndLine:   7,
				Severity:  domain.FindingSeverityMajor,
				Title:     "Nil risk",
				Detail:    "Potential nil dereference.",
				SuggestedChange: &domain.SuggestedChange{
					StartLine:   20,
					EndLine:     19,
					Kind:        domain.SuggestedChangeKindReplace,
					Replacement: "if value == nil {\n\treturn err\n}",
				},
			},
		},
		Summary: "done",
	})
	require.NoError(t, err)
	require.Empty(t, client.reviewInputs)
	require.Len(t, client.bodies, 1)
	require.True(t, containsEvent(logger.events, "warn:Skipped one GitHub review comment because its anchor is invalid."))
}

func TestPublisher_PublishSkipsInvalidLocalAnchor(t *testing.T) {
	client := &fakeClient{}
	logger := &spyLogger{}
	pub := NewPublisher(client, logger)

	err := pub.Publish(context.Background(), usecase.ReviewPublishResult{
		Target: domain.ChangeRequestTarget{
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
	require.True(t, containsEvent(logger.events, "debug:GitHub review comment metadata state=\"skipped_invalid_anchor\""))
	require.True(t, containsEvent(logger.events, "trace:GitHub review comment content state=\"skipped_invalid_anchor\""))
}

func TestPublisher_PublishSkipsInvalidAnchorFromClient(t *testing.T) {
	client := &fakeClient{
		reviewErr: &domain.InvalidAnchorError{Message: "invalid review comment anchor"},
	}
	logger := &spyLogger{}
	pub := NewPublisher(client, logger)

	err := pub.Publish(context.Background(), usecase.ReviewPublishResult{
		Target: domain.ChangeRequestTarget{
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
	require.True(t, containsEvent(logger.events, "debug:GitHub review comment metadata state=\"skipped_invalid_anchor\""))
	require.True(t, containsEvent(logger.events, "trace:GitHub review comment content state=\"skipped_invalid_anchor\""))
}

func TestPublisher_PublishFailsForNonAnchorError(t *testing.T) {
	client := &fakeClient{
		reviewErr: errors.New("network fail"),
	}
	logger := &spyLogger{}
	pub := NewPublisher(client, logger)

	err := pub.Publish(context.Background(), usecase.ReviewPublishResult{
		Target: domain.ChangeRequestTarget{
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
	require.True(t, containsEvent(logger.events, "debug:GitHub review comment metadata state=\"failed\""))
	require.True(t, containsEvent(logger.events, "trace:GitHub review comment content state=\"failed\""))
}

func TestPublisher_PublishFailsForSummaryError(t *testing.T) {
	client := &fakeClient{
		commentErr: errors.New("summary fail"),
	}
	logger := &spyLogger{}
	pub := NewPublisher(client, logger)

	err := pub.Publish(context.Background(), usecase.ReviewPublishResult{
		Target: domain.ChangeRequestTarget{
			Repository:          "org/repo",
			ChangeRequestNumber: 11,
		},
		Summary: "done",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "summary fail")
	require.True(t, containsEvent(logger.events, "debug:GitHub review summary metadata state=\"failed\""))
	require.True(t, containsEvent(logger.events, "trace:GitHub review summary content state=\"failed\""))
}

func TestPublisher_PublishPostsOnlySummaryWhenNoFindings(t *testing.T) {
	client := &fakeClient{}
	pub := NewPublisher(client, nil)

	err := pub.Publish(context.Background(), usecase.ReviewPublishResult{
		Target: domain.ChangeRequestTarget{
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
