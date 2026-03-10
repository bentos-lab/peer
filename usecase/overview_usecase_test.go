package usecase

import (
	"context"
	"errors"
	"testing"

	"bentos-backend/domain"
	uccontracts "bentos-backend/usecase/contracts"

	"github.com/stretchr/testify/require"
)

type overviewUseCaseTestGenerator struct {
	lastPayload LLMOverviewPayload
	callCount   int
	result      LLMOverviewResult
	err         error
}

func (g *overviewUseCaseTestGenerator) GenerateOverview(_ context.Context, payload LLMOverviewPayload) (LLMOverviewResult, error) {
	g.callCount++
	g.lastPayload = payload
	if g.err != nil {
		return LLMOverviewResult{}, g.err
	}
	return g.result, nil
}

type overviewUseCaseTestPublisher struct {
	lastRequest OverviewPublishRequest
	callCount   int
	err         error
}

func (p *overviewUseCaseTestPublisher) PublishOverview(_ context.Context, req OverviewPublishRequest) error {
	p.callCount++
	p.lastRequest = req
	return p.err
}

type overviewUseCaseTestEnvironment struct {
	cleanupCalls int
	cleanupErr   error
}

func (e *overviewUseCaseTestEnvironment) SetupAgent(_ context.Context, _ domain.CodingAgentSetupOptions) (uccontracts.CodingAgent, error) {
	return nil, errors.New("not implemented")
}

func (e *overviewUseCaseTestEnvironment) LoadChangedFiles(_ context.Context, _ domain.CodeEnvironmentLoadOptions) ([]domain.ChangedFile, error) {
	return nil, errors.New("not implemented")
}

func (e *overviewUseCaseTestEnvironment) Cleanup(_ context.Context) error {
	e.cleanupCalls++
	return e.cleanupErr
}

type overviewUseCaseTestEnvironmentFactory struct {
	environment uccontracts.CodeEnvironment
	lastOpts    domain.CodeEnvironmentInitOptions
	callCount   int
	err         error
}

func (f *overviewUseCaseTestEnvironmentFactory) New(_ context.Context, opts domain.CodeEnvironmentInitOptions) (uccontracts.CodeEnvironment, error) {
	f.callCount++
	f.lastOpts = opts
	if f.err != nil {
		return nil, f.err
	}
	return f.environment, nil
}

func TestOverviewUseCaseExecuteInitializesEnvironmentAndPassesItToGenerator(t *testing.T) {
	environment := &overviewUseCaseTestEnvironment{}
	factory := &overviewUseCaseTestEnvironmentFactory{environment: environment}
	generator := &overviewUseCaseTestGenerator{
		result: LLMOverviewResult{
			Categories: []domain.OverviewCategoryItem{{
				Category: domain.OverviewCategoryLogicUpdates,
				Summary:  "logic updates",
			}},
		},
	}
	useCase, err := NewOverviewUseCase(generator, &overviewUseCaseTestPublisher{}, factory, nil)
	require.NoError(t, err)

	_, err = useCase.Execute(context.Background(), OverviewRequest{
		Input: domain.ChangeRequestInput{
			Target:  domain.ChangeRequestTarget{Repository: "org/repo", ChangeRequestNumber: 42},
			RepoURL: "https://github.com/org/repo.git",
			Base:    "main",
			Head:    "feature",
		},
	})
	require.NoError(t, err)
	require.Equal(t, 1, factory.callCount)
	require.Equal(t, domain.CodeEnvironmentInitOptions{
		RepoURL: "https://github.com/org/repo.git",
	}, factory.lastOpts)
	require.Equal(t, 1, generator.callCount)
	require.Same(t, environment, generator.lastPayload.Environment)
}

func TestOverviewUseCaseExecuteReturnsFactoryError(t *testing.T) {
	factory := &overviewUseCaseTestEnvironmentFactory{err: errors.New("factory failed")}
	generator := &overviewUseCaseTestGenerator{}
	useCase, err := NewOverviewUseCase(generator, &overviewUseCaseTestPublisher{}, factory, nil)
	require.NoError(t, err)

	_, err = useCase.Execute(context.Background(), OverviewRequest{
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
	require.Equal(t, 0, generator.callCount)
}

func TestOverviewUseCaseExecuteCleansUpEnvironmentOnSuccess(t *testing.T) {
	environment := &overviewUseCaseTestEnvironment{}
	factory := &overviewUseCaseTestEnvironmentFactory{environment: environment}
	generator := &overviewUseCaseTestGenerator{
		result: LLMOverviewResult{
			Categories: []domain.OverviewCategoryItem{{
				Category: domain.OverviewCategoryLogicUpdates,
				Summary:  "logic updates",
			}},
		},
	}
	useCase, err := NewOverviewUseCase(generator, &overviewUseCaseTestPublisher{}, factory, nil)
	require.NoError(t, err)

	_, err = useCase.Execute(context.Background(), OverviewRequest{
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

func TestOverviewUseCaseExecuteCleansUpEnvironmentOnGeneratorError(t *testing.T) {
	environment := &overviewUseCaseTestEnvironment{}
	factory := &overviewUseCaseTestEnvironmentFactory{environment: environment}
	generator := &overviewUseCaseTestGenerator{err: errors.New("generator failed")}
	useCase, err := NewOverviewUseCase(generator, &overviewUseCaseTestPublisher{}, factory, nil)
	require.NoError(t, err)

	_, err = useCase.Execute(context.Background(), OverviewRequest{
		Input: domain.ChangeRequestInput{
			Target:  domain.ChangeRequestTarget{Repository: "org/repo", ChangeRequestNumber: 42},
			RepoURL: "https://github.com/org/repo.git",
			Base:    "main",
			Head:    "feature",
		},
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "generator failed")
	require.Equal(t, 1, environment.cleanupCalls)
}
