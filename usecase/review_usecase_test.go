package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"bentos-backend/domain"
	"github.com/stretchr/testify/require"
)

type mockInputProvider struct {
	snapshot domain.ChangeSnapshot
	err      error
}

func (m mockInputProvider) LoadChangeSnapshot(_ context.Context, _ ChangeRequestRequest) (domain.ChangeSnapshot, error) {
	return m.snapshot, m.err
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

type mockOverviewLLM struct {
	result LLMOverviewResult
	err    error
	calls  int
}

func (m *mockOverviewLLM) GenerateOverview(_ context.Context, _ LLMOverviewPayload) (LLMOverviewResult, error) {
	m.calls++
	return m.result, m.err
}

type mockOverviewPublisher struct {
	last OverviewPublishRequest
	err  error
}

func (m *mockOverviewPublisher) PublishOverview(_ context.Context, req OverviewPublishRequest) error {
	m.last = req
	return m.err
}

type blockingOverviewUseCase struct {
	started   chan struct{}
	release   chan struct{}
	executed  atomic.Int32
	result    OverviewExecutionResult
	err       error
	startOnce sync.Once
}

func (b *blockingOverviewUseCase) Execute(_ context.Context, _ OverviewRequest) (OverviewExecutionResult, error) {
	b.executed.Add(1)
	b.startOnce.Do(func() {
		close(b.started)
	})
	<-b.release
	return b.result, b.err
}

type spyReviewUseCase struct {
	started   chan struct{}
	executed  atomic.Int32
	result    ReviewExecutionResult
	err       error
	startOnce sync.Once
}

func (s *spyReviewUseCase) Execute(_ context.Context, _ ReviewRequest) (ReviewExecutionResult, error) {
	s.executed.Add(1)
	s.startOnce.Do(func() {
		close(s.started)
	})
	return s.result, s.err
}

type spyLogEvent struct {
	level string
	msg   string
}

type spyLogger struct {
	events []spyLogEvent
}

func (s *spyLogger) Infof(format string, args ...any) {
	s.events = append(s.events, spyLogEvent{level: "info", msg: fmt.Sprintf(format, args...)})
}

func (s *spyLogger) Debugf(format string, args ...any) {
	s.events = append(s.events, spyLogEvent{level: "debug", msg: fmt.Sprintf(format, args...)})
}

func (s *spyLogger) Warnf(format string, args ...any) {
	s.events = append(s.events, spyLogEvent{level: "warn", msg: fmt.Sprintf(format, args...)})
}

func (s *spyLogger) Errorf(format string, args ...any) {
	s.events = append(s.events, spyLogEvent{level: "error", msg: fmt.Sprintf(format, args...)})
}

func TestChangeRequestUseCase_ExecutePublishesMessages(t *testing.T) {
	llm := &mockLLM{
		result: LLMReviewResult{
			Findings: []domain.Finding{{
				FilePath:  "service.go",
				StartLine: 12,
				EndLine:   12,
				Severity:  domain.FindingSeverityMajor,
				Title:     "Error handling",
				Detail:    "Returned error is ignored.",
			}},
			Summary: "1 important potential issue found.",
		},
	}
	reviewPub := &mockPublisher{}
	overviewLLM := &mockOverviewLLM{
		result: LLMOverviewResult{
			Categories: []domain.OverviewCategoryItem{{Category: domain.OverviewCategoryLogicUpdates, Summary: "Updated validation path."}},
		},
	}
	overviewPub := &mockOverviewPublisher{}
	logger := &spyLogger{}

	reviewUC, err := NewReviewUseCase(
		mockRuleProvider{pack: RulePack{ID: "core", Version: "v1", Name: "Core", Instructions: []string{"review bug risks"}}},
		llm,
		reviewPub,
		logger,
	)
	require.NoError(t, err)
	overviewUC, err := NewOverviewUseCase(overviewLLM, overviewPub, logger)
	require.NoError(t, err)
	uc, err := NewChangeRequestUseCase(
		mockInputProvider{snapshot: domain.ChangeSnapshot{
			Context:      domain.ChangeRequestContext{Repository: "org/repo", ChangeRequestNumber: 10, Metadata: map[string]string{"action": "opened"}},
			ChangedFiles: []domain.ChangedFile{{Path: "service.go", DiffSnippet: "@@"}},
		}},
		reviewUC,
		overviewUC,
		logger,
	)
	require.NoError(t, err)

	result, err := uc.Execute(context.Background(), ChangeRequestRequest{EnableOverview: true, Metadata: map[string]string{"action": "opened"}})
	require.NoError(t, err)
	require.Len(t, result.Messages, 2)
	require.Equal(t, domain.ReviewMessageTypeSummary, result.Messages[1].Type)
	require.Equal(t, 1, llm.calls)
	require.Equal(t, "org/repo", reviewPub.last.Target.Repository)
	require.Equal(t, llm.result.Findings, reviewPub.last.Findings)
	require.Equal(t, llm.result.Summary, reviewPub.last.Summary)
	require.Equal(t, "org/repo", overviewPub.last.Target.Repository)
	require.Len(t, result.Overview.Categories, 1)
	require.GreaterOrEqual(t, len(logger.events), 10)
	require.True(t, containsUsecaseEvent(logger.events, "info", "Review execution started."))
	require.True(t, containsUsecaseEvent(logger.events, "info", "Review execution completed."))
}

func TestReviewUseCase_ExecuteLogsStageFailure(t *testing.T) {
	expectedErr := errors.New("publish failed")
	llm := &mockLLM{result: LLMReviewResult{Summary: "summary", Findings: []domain.Finding{}}}
	pub := &mockPublisher{err: expectedErr}
	logger := &spyLogger{}

	uc, err := NewReviewUseCase(
		mockRuleProvider{pack: RulePack{ID: "core", Instructions: []string{"review"}}},
		llm,
		pub,
		logger,
	)
	require.NoError(t, err)

	_, err = uc.Execute(context.Background(), ReviewRequest{Input: domain.ReviewInput{Target: domain.ReviewTarget{Repository: "org/repo", ChangeRequestNumber: 10}}})
	require.Error(t, err)
	require.ErrorIs(t, err, expectedErr)
	require.True(t, containsUsecaseEvent(logger.events, "error", "Review stage failed."))
	require.True(t, containsUsecaseEvent(logger.events, "debug", "Stage \"publish_review_result\" failed"))
}

func TestChangeRequestUseCase_ExecuteFailsWhenOverviewGenerationFails(t *testing.T) {
	expectedErr := errors.New("overview generation failed")
	llm := &mockLLM{result: LLMReviewResult{Summary: "summary"}}
	overviewLLM := &mockOverviewLLM{err: expectedErr}
	reviewPub := &mockPublisher{}
	overviewPub := &mockOverviewPublisher{}
	logger := &spyLogger{}

	reviewUC, err := NewReviewUseCase(
		mockRuleProvider{pack: RulePack{ID: "core", Instructions: []string{"review"}}},
		llm,
		reviewPub,
		logger,
	)
	require.NoError(t, err)
	overviewUC, err := NewOverviewUseCase(overviewLLM, overviewPub, logger)
	require.NoError(t, err)
	uc, err := NewChangeRequestUseCase(
		mockInputProvider{snapshot: domain.ChangeSnapshot{Context: domain.ChangeRequestContext{Repository: "org/repo", ChangeRequestNumber: 10}}},
		reviewUC,
		overviewUC,
		logger,
	)
	require.NoError(t, err)

	_, err = uc.Execute(context.Background(), ChangeRequestRequest{EnableOverview: true})
	require.ErrorIs(t, err, expectedErr)
	require.Equal(t, 0, llm.calls)
	require.True(t, containsUsecaseEvent(logger.events, "debug", "generate_overview"))
}

func TestChangeRequestUseCase_ExecuteRunsOverviewBeforeReview(t *testing.T) {
	logger := &spyLogger{}
	reviewUC := &spyReviewUseCase{
		started: make(chan struct{}),
		result: ReviewExecutionResult{
			Summary: "review summary",
		},
	}
	overviewUC := &blockingOverviewUseCase{
		started: make(chan struct{}),
		release: make(chan struct{}),
		result: OverviewExecutionResult{
			Overview: LLMOverviewResult{
				Categories: []domain.OverviewCategoryItem{
					{Category: domain.OverviewCategoryLogicUpdates, Summary: "overview summary"},
				},
			},
		},
	}

	uc, err := NewChangeRequestUseCase(
		mockInputProvider{
			snapshot: domain.ChangeSnapshot{
				Context: domain.ChangeRequestContext{
					Repository:          "org/repo",
					ChangeRequestNumber: 10,
				},
			},
		},
		reviewUC,
		overviewUC,
		logger,
	)
	require.NoError(t, err)

	done := make(chan struct{})
	var result ChangeRequestExecutionResult
	var executeErr error
	go func() {
		result, executeErr = uc.Execute(context.Background(), ChangeRequestRequest{EnableOverview: true})
		close(done)
	}()

	select {
	case <-overviewUC.started:
	case <-time.After(2 * time.Second):
		t.Fatal("expected overview execution to start")
	}

	select {
	case <-reviewUC.started:
		t.Fatal("review must not start before overview completes")
	default:
	}

	close(overviewUC.release)

	select {
	case <-reviewUC.started:
	case <-time.After(2 * time.Second):
		t.Fatal("expected review execution to start after overview completion")
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("expected Execute to complete")
	}

	require.NoError(t, executeErr)
	require.Equal(t, int32(1), overviewUC.executed.Load())
	require.Equal(t, int32(1), reviewUC.executed.Load())
	require.Equal(t, "review summary", result.Summary)
	require.Len(t, result.Overview.Categories, 1)
}

func containsUsecaseEvent(events []spyLogEvent, level string, target string) bool {
	for _, event := range events {
		if event.level == level && strings.Contains(event.msg, target) {
			return true
		}
	}
	return false
}
