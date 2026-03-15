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

type overviewUseCaseTestIssueAlignmentGenerator struct {
	lastPayload LLMIssueAlignmentPayload
	callCount   int
	result      domain.IssueAlignmentResult
	err         error
}

func (g *overviewUseCaseTestIssueAlignmentGenerator) GenerateIssueAlignment(_ context.Context, payload LLMIssueAlignmentPayload) (domain.IssueAlignmentResult, error) {
	g.callCount++
	g.lastPayload = payload
	if g.err != nil {
		return domain.IssueAlignmentResult{}, g.err
	}
	return g.result, nil
}

type overviewUseCaseTestEnvironment struct {
}

func (e *overviewUseCaseTestEnvironment) SetupAgent(_ context.Context, _ domain.CodingAgentSetupOptions) (uccontracts.CodingAgent, error) {
	return nil, errors.New("not implemented")
}

func (e *overviewUseCaseTestEnvironment) LoadChangedFiles(_ context.Context, _ domain.CodeEnvironmentLoadOptions) ([]domain.ChangedFile, error) {
	return nil, errors.New("not implemented")
}

func (e *overviewUseCaseTestEnvironment) ReadFile(_ context.Context, _ string, _ string) (string, bool, error) {
	return "", false, errors.New("not implemented")
}

func (e *overviewUseCaseTestEnvironment) PushChanges(_ context.Context, _ domain.CodeEnvironmentPushOptions) (domain.CodeEnvironmentPushResult, error) {
	return domain.CodeEnvironmentPushResult{}, errors.New("not implemented")
}

func (e *overviewUseCaseTestEnvironment) Cleanup(_ context.Context) error {
	return nil
}

func TestOverviewUseCaseExecutePassesEnvironmentToGenerator(t *testing.T) {
	environment := &overviewUseCaseTestEnvironment{}
	generator := &overviewUseCaseTestGenerator{
		result: LLMOverviewResult{
			Categories: []domain.OverviewCategoryItem{{
				Category: domain.OverviewCategoryLogicUpdates,
				Summary:  "logic updates",
			}},
		},
	}
	issueAlignment := &overviewUseCaseTestIssueAlignmentGenerator{}
	useCase, err := NewOverviewUseCase(generator, issueAlignment, &overviewUseCaseTestPublisher{}, nil)
	require.NoError(t, err)

	_, err = useCase.Execute(context.Background(), OverviewRequest{
		Input: domain.ChangeRequestInput{
			Target:  domain.ChangeRequestTarget{Repository: "org/repo", ChangeRequestNumber: 42},
			RepoURL: "https://github.com/org/repo.git",
			Base:    "main",
			Head:    "feature",
		},
		Environment: environment,
	})
	require.NoError(t, err)
	require.Equal(t, 1, generator.callCount)
	require.Same(t, environment, generator.lastPayload.Environment)
}

func TestOverviewUseCaseRequiresEnvironment(t *testing.T) {
	generator := &overviewUseCaseTestGenerator{}
	issueAlignment := &overviewUseCaseTestIssueAlignmentGenerator{}
	useCase, err := NewOverviewUseCase(generator, issueAlignment, &overviewUseCaseTestPublisher{}, nil)
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
	require.ErrorContains(t, err, "code environment")
	require.Equal(t, 0, generator.callCount)
}
