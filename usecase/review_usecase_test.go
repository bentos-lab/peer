package usecase

import (
	"context"
	"errors"
	"testing"

	"bentos-backend/domain"
	uccontracts "bentos-backend/usecase/contracts"

	"github.com/stretchr/testify/require"
)

type reviewUseCaseTestRulePackProvider struct {
	pack RulePack
	err  error
}

func (p *reviewUseCaseTestRulePackProvider) CorePack(_ context.Context) (RulePack, error) {
	if p.err != nil {
		return RulePack{}, p.err
	}
	return p.pack, nil
}

type reviewUseCaseTestReviewer struct {
	lastPayload LLMReviewPayload
	callCount   int
	result      LLMReviewResult
	err         error
}

func (r *reviewUseCaseTestReviewer) Review(_ context.Context, payload LLMReviewPayload) (LLMReviewResult, error) {
	r.callCount++
	r.lastPayload = payload
	if r.err != nil {
		return LLMReviewResult{}, r.err
	}
	return r.result, nil
}

type reviewUseCaseTestPublisher struct {
	lastResult ReviewPublishResult
	callCount  int
	err        error
}

func (p *reviewUseCaseTestPublisher) Publish(_ context.Context, result ReviewPublishResult) error {
	p.callCount++
	p.lastResult = result
	return p.err
}

type reviewUseCaseTestEnvironment struct {
}

func (e *reviewUseCaseTestEnvironment) SetupAgent(_ context.Context, _ domain.CodingAgentSetupOptions) (uccontracts.CodingAgent, error) {
	return nil, errors.New("not implemented")
}

func (e *reviewUseCaseTestEnvironment) LoadChangedFiles(_ context.Context, _ domain.CodeEnvironmentLoadOptions) ([]domain.ChangedFile, error) {
	return nil, errors.New("not implemented")
}

func (e *reviewUseCaseTestEnvironment) ReadFile(_ context.Context, _ string, _ string) (string, bool, error) {
	return "", false, errors.New("not implemented")
}

func (e *reviewUseCaseTestEnvironment) PushChanges(_ context.Context, _ domain.CodeEnvironmentPushOptions) (domain.CodeEnvironmentPushResult, error) {
	return domain.CodeEnvironmentPushResult{}, errors.New("not implemented")
}

func (e *reviewUseCaseTestEnvironment) Cleanup(_ context.Context) error {
	return nil
}

func TestReviewUseCaseExecutePassesEnvironmentToReviewer(t *testing.T) {
	environment := &reviewUseCaseTestEnvironment{}
	reviewer := &reviewUseCaseTestReviewer{
		result: LLMReviewResult{
			Summary: "summary",
			Findings: []domain.Finding{{
				FilePath:   "a.go",
				StartLine:  1,
				EndLine:    1,
				Severity:   domain.FindingSeverityMinor,
				Title:      "title",
				Detail:     "detail",
				Suggestion: "suggestion",
			}},
		},
	}
	useCase, err := NewReviewUseCase(
		&reviewUseCaseTestRulePackProvider{pack: RulePack{Instructions: []string{"rule-1"}}},
		reviewer,
		&reviewUseCaseTestPublisher{},
		nil,
	)
	require.NoError(t, err)

	_, err = useCase.Execute(context.Background(), ReviewRequest{
		Input: domain.ChangeRequestInput{
			Target:  domain.ChangeRequestTarget{Repository: "org/repo", ChangeRequestNumber: 42},
			RepoURL: "https://github.com/org/repo.git",
			Base:    "main",
			Head:    "feature",
		},
		Suggestions: true,
		Environment: environment,
	})
	require.NoError(t, err)
	require.Equal(t, 1, reviewer.callCount)
	require.Same(t, environment, reviewer.lastPayload.Environment)
	require.True(t, reviewer.lastPayload.Suggestions)
}

func TestReviewUseCaseRequiresEnvironment(t *testing.T) {
	reviewer := &reviewUseCaseTestReviewer{}
	useCase, err := NewReviewUseCase(
		&reviewUseCaseTestRulePackProvider{pack: RulePack{Instructions: []string{"rule-1"}}},
		reviewer,
		&reviewUseCaseTestPublisher{},
		nil,
	)
	require.NoError(t, err)

	_, err = useCase.Execute(context.Background(), ReviewRequest{
		Input: domain.ChangeRequestInput{
			Target:  domain.ChangeRequestTarget{Repository: "org/repo", ChangeRequestNumber: 42},
			RepoURL: "https://github.com/org/repo.git",
			Base:    "main",
			Head:    "feature",
		},
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "code environment")
	require.Equal(t, 0, reviewer.callCount)
}

func TestReviewUseCaseExecutePassesSuggestionsDisabledToReviewer(t *testing.T) {
	environment := &reviewUseCaseTestEnvironment{}
	reviewer := &reviewUseCaseTestReviewer{
		result: LLMReviewResult{
			Summary:  "summary",
			Findings: []domain.Finding{},
		},
	}
	useCase, err := NewReviewUseCase(
		&reviewUseCaseTestRulePackProvider{pack: RulePack{Instructions: []string{"rule-1"}}},
		reviewer,
		&reviewUseCaseTestPublisher{},
		nil,
	)
	require.NoError(t, err)

	_, err = useCase.Execute(context.Background(), ReviewRequest{
		Input: domain.ChangeRequestInput{
			Target:  domain.ChangeRequestTarget{Repository: "org/repo", ChangeRequestNumber: 42},
			RepoURL: "https://github.com/org/repo.git",
			Base:    "main",
			Head:    "feature",
		},
		Suggestions: false,
		Environment: environment,
	})
	require.NoError(t, err)
	require.Equal(t, 1, reviewer.callCount)
	require.False(t, reviewer.lastPayload.Suggestions)
}
