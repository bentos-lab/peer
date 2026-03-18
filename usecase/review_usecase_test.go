package usecase

import (
	"context"
	"errors"
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

func TestReviewerUseCase_ExecutePublishesMessages(t *testing.T) {
	llm := &mockLLM{
		result: LLMReviewResult{
			Findings: []domain.Finding{
				{
					FilePath: "service.go",
					Line:     12,
					Severity: domain.FindingSeverityMajor,
					Title:    "Error handling",
					Detail:   "Returned error is ignored.",
				},
			},
			Summary: "1 important potential issue found.",
		},
	}
	pub := &mockPublisher{}
	uc, err := NewReviewerUseCase(
		mockInputProvider{
			input: domain.ReviewInput{
				ReviewID: "r1",
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
	)
	require.NoError(t, err)

	result, err := uc.Execute(context.Background(), ReviewRequest{ReviewID: "r1"})
	require.NoError(t, err)
	require.Len(t, result.Messages, 2)
	require.Equal(t, domain.ReviewMessageTypeSummary, result.Messages[1].Type)
	require.Equal(t, 1, llm.calls)
	require.Equal(t, "org/repo", pub.last.Target.Repository)
}
