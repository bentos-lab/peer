package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"bentos-backend/domain"
	"github.com/stretchr/testify/require"
)

type mockInputProvider struct {
	input domain.ReviewInput
	err   error
}

func (m mockInputProvider) LoadReviewInput(_ context.Context, _ ReviewRequest) (domain.ReviewInput, error) {
	return m.input, m.err
}

type mockRuleProvider struct {
	pack RulePack
	err  error
}

func (m mockRuleProvider) CorePack(_ context.Context) (RulePack, error) {
	return m.pack, m.err
}

type mockLLM struct {
	result LLMReviewResult
	err    error
	calls  int
}

func (m *mockLLM) ReviewDiff(_ context.Context, payload LLMReviewPayload) (LLMReviewResult, error) {
	m.calls++
	if payload.RulePack.ID == "" {
		return LLMReviewResult{}, errors.New("missing rule pack")
	}
	return m.result, m.err
}

type mockPublisher struct {
	last ReviewPublishResult
	err  error
}

func (m *mockPublisher) Publish(_ context.Context, result ReviewPublishResult) error {
	m.last = result
	return m.err
}

type spyLogEvent struct {
	level string
	msg   string
}

type spyLogger struct {
	events []spyLogEvent
}

func (s *spyLogger) Infof(format string, args ...any) {
	s.events = append(s.events, spyLogEvent{
		level: "info",
		msg:   fmt.Sprintf(format, args...),
	})
}

func (s *spyLogger) Debugf(format string, args ...any) {
	s.events = append(s.events, spyLogEvent{
		level: "debug",
		msg:   fmt.Sprintf(format, args...),
	})
}

func (s *spyLogger) Errorf(format string, args ...any) {
	s.events = append(s.events, spyLogEvent{
		level: "error",
		msg:   fmt.Sprintf(format, args...),
	})
}

func TestReviewerUseCase_ExecutePublishesMessages(t *testing.T) {
	llm := &mockLLM{
		result: LLMReviewResult{
			Findings: []domain.Finding{
				{
					FilePath:  "service.go",
					StartLine: 12,
					EndLine:   12,
					Severity:  domain.FindingSeverityMajor,
					Title:     "Error handling",
					Detail:    "Returned error is ignored.",
				},
			},
			Summary: "1 important potential issue found.",
		},
	}
	pub := &mockPublisher{}
	logger := &spyLogger{}
	uc, err := NewReviewerUseCase(
		mockInputProvider{
			input: domain.ReviewInput{
				Target: domain.ReviewTarget{
					Repository:          "org/repo",
					ChangeRequestNumber: 10,
				},
				ChangedFiles: []domain.ChangedFile{
					{Path: "service.go", DiffSnippet: "@@"},
				},
			},
		},
		mockRuleProvider{
			pack: RulePack{
				ID:           "core",
				Version:      "v1",
				Name:         "Core",
				Instructions: []string{"review bug risks"},
			},
		},
		llm,
		pub,
		logger,
	)
	require.NoError(t, err)

	result, err := uc.Execute(context.Background(), ReviewRequest{})
	require.NoError(t, err)
	require.Len(t, result.Messages, 2)
	require.Equal(t, domain.ReviewMessageTypeSummary, result.Messages[1].Type)
	require.Equal(t, 1, llm.calls)
	require.Equal(t, "org/repo", pub.last.Target.Repository)
	require.Equal(t, llm.result.Findings, pub.last.Findings)
	require.Equal(t, llm.result.Summary, pub.last.Summary)
	require.GreaterOrEqual(t, len(logger.events), 10)
	require.True(t, containsUsecaseEvent(logger.events, "info", "Review execution started."))
	require.True(t, containsUsecaseEvent(logger.events, "info", "Review execution completed."))
}

func TestReviewerUseCase_ExecuteLogsStageFailure(t *testing.T) {
	expectedErr := errors.New("publish failed")
	llm := &mockLLM{
		result: LLMReviewResult{
			Summary:  "summary",
			Findings: []domain.Finding{},
		},
	}
	pub := &mockPublisher{err: expectedErr}
	logger := &spyLogger{}

	uc, err := NewReviewerUseCase(
		mockInputProvider{
			input: domain.ReviewInput{
				Target: domain.ReviewTarget{
					Repository:          "org/repo",
					ChangeRequestNumber: 10,
				},
			},
		},
		mockRuleProvider{
			pack: RulePack{
				ID:           "core",
				Instructions: []string{"review"},
			},
		},
		llm,
		pub,
		logger,
	)
	require.NoError(t, err)

	_, err = uc.Execute(context.Background(), ReviewRequest{Repository: "org/repo", ChangeRequestNumber: 10})
	require.Error(t, err)
	require.ErrorIs(t, err, expectedErr)
	require.True(t, containsUsecaseEvent(logger.events, "error", "Review stage failed."))
	require.True(t, containsUsecaseEvent(logger.events, "debug", "Stage \"publish_review_result\" failed"))
}

func containsUsecaseEvent(events []spyLogEvent, level string, target string) bool {
	for _, event := range events {
		if event.level == level && strings.Contains(event.msg, target) {
			return true
		}
	}
	return false
}
