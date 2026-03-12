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
	cleanupCalls int
	cleanupErr   error
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
	e.cleanupCalls++
	return e.cleanupErr
}

type reviewUseCaseTestEnvironmentFactory struct {
	environment uccontracts.CodeEnvironment
	lastOpts    domain.CodeEnvironmentInitOptions
	callCount   int
	err         error
}

func (f *reviewUseCaseTestEnvironmentFactory) New(_ context.Context, opts domain.CodeEnvironmentInitOptions) (uccontracts.CodeEnvironment, error) {
	f.callCount++
	f.lastOpts = opts
	if f.err != nil {
		return nil, f.err
	}
	return f.environment, nil
}

func TestReviewUseCaseExecuteInitializesEnvironmentAndPassesItToReviewer(t *testing.T) {
	environment := &reviewUseCaseTestEnvironment{}
	factory := &reviewUseCaseTestEnvironmentFactory{environment: environment}
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
		factory,
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
	})
	require.NoError(t, err)
	require.Equal(t, 1, factory.callCount)
	require.Equal(t, domain.CodeEnvironmentInitOptions{
		RepoURL: "https://github.com/org/repo.git",
	}, factory.lastOpts)
	require.Equal(t, 1, reviewer.callCount)
	require.Same(t, environment, reviewer.lastPayload.Environment)
	require.True(t, reviewer.lastPayload.Suggestions)
}

func TestReviewUseCaseExecuteReturnsFactoryError(t *testing.T) {
	factory := &reviewUseCaseTestEnvironmentFactory{err: errors.New("factory failed")}
	reviewer := &reviewUseCaseTestReviewer{}
	useCase, err := NewReviewUseCase(
		&reviewUseCaseTestRulePackProvider{pack: RulePack{Instructions: []string{"rule-1"}}},
		reviewer,
		&reviewUseCaseTestPublisher{},
		factory,
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
	require.ErrorContains(t, err, "factory failed")
	require.Equal(t, 1, factory.callCount)
	require.Equal(t, 0, reviewer.callCount)
}

func TestReviewUseCaseExecutePassesSuggestionsDisabledToReviewer(t *testing.T) {
	environment := &reviewUseCaseTestEnvironment{}
	factory := &reviewUseCaseTestEnvironmentFactory{environment: environment}
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
		factory,
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
	})
	require.NoError(t, err)
	require.Equal(t, 1, reviewer.callCount)
	require.False(t, reviewer.lastPayload.Suggestions)
}

func TestReviewUseCaseExecuteCleansUpEnvironmentOnSuccess(t *testing.T) {
	environment := &reviewUseCaseTestEnvironment{}
	factory := &reviewUseCaseTestEnvironmentFactory{environment: environment}
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
		factory,
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
	require.NoError(t, err)
	require.Equal(t, 1, environment.cleanupCalls)
}

func TestReviewUseCaseExecuteCleansUpEnvironmentOnReviewerError(t *testing.T) {
	environment := &reviewUseCaseTestEnvironment{}
	factory := &reviewUseCaseTestEnvironmentFactory{environment: environment}
	reviewer := &reviewUseCaseTestReviewer{err: errors.New("review failed")}
	useCase, err := NewReviewUseCase(
		&reviewUseCaseTestRulePackProvider{pack: RulePack{Instructions: []string{"rule-1"}}},
		reviewer,
		&reviewUseCaseTestPublisher{},
		factory,
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
	require.ErrorContains(t, err, "review failed")
	require.Equal(t, 1, environment.cleanupCalls)
}
