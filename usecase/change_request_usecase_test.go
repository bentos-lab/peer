package usecase

import (
	"context"
	"errors"
	"testing"

	"bentos-backend/domain"
	uccontracts "bentos-backend/usecase/contracts"
	"github.com/stretchr/testify/require"
)

type changeRequestTestEnv struct{}

func (e *changeRequestTestEnv) SetupAgent(_ context.Context, _ domain.CodingAgentSetupOptions) (uccontracts.CodingAgent, error) {
	return nil, nil
}

func (e *changeRequestTestEnv) LoadChangedFiles(_ context.Context, _ domain.CodeEnvironmentLoadOptions) ([]domain.ChangedFile, error) {
	return nil, nil
}

func (e *changeRequestTestEnv) ReadFile(_ context.Context, _ string, _ string) (string, bool, error) {
	return "", false, nil
}

func (e *changeRequestTestEnv) PushChanges(_ context.Context, _ domain.CodeEnvironmentPushOptions) (domain.CodeEnvironmentPushResult, error) {
	return domain.CodeEnvironmentPushResult{}, nil
}

func (e *changeRequestTestEnv) Cleanup(_ context.Context) error {
	return nil
}

type changeRequestTestReviewUseCase struct {
	calls     int
	lastReq   ReviewRequest
	resultErr error
}

func (u *changeRequestTestReviewUseCase) Execute(_ context.Context, req ReviewRequest) (ReviewExecutionResult, error) {
	u.calls++
	u.lastReq = req
	return ReviewExecutionResult{}, u.resultErr
}

type changeRequestTestOverviewUseCase struct {
	calls     int
	lastReq   OverviewRequest
	resultErr error
}

func (u *changeRequestTestOverviewUseCase) Execute(_ context.Context, req OverviewRequest) (OverviewExecutionResult, error) {
	u.calls++
	u.lastReq = req
	return OverviewExecutionResult{}, u.resultErr
}

func TestChangeRequestUseCaseRequiresEnvironmentWhenEnabled(t *testing.T) {
	reviewUseCase := &changeRequestTestReviewUseCase{}
	overviewUseCase := &changeRequestTestOverviewUseCase{}
	useCase, err := NewChangeRequestUseCase(reviewUseCase, overviewUseCase, nil)
	require.NoError(t, err)

	_, err = useCase.Execute(context.Background(), ChangeRequestRequest{
		Repository:        "org/repo",
		EnableReview:      true,
		EnableOverview:    false,
		EnableSuggestions: false,
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "code environment")
	require.Equal(t, 0, reviewUseCase.calls)
}

func TestChangeRequestUseCasePassesEnvironmentToSubUseCases(t *testing.T) {
	environment := &changeRequestTestEnv{}
	reviewUseCase := &changeRequestTestReviewUseCase{}
	overviewUseCase := &changeRequestTestOverviewUseCase{}
	useCase, err := NewChangeRequestUseCase(reviewUseCase, overviewUseCase, nil)
	require.NoError(t, err)

	_, err = useCase.Execute(context.Background(), ChangeRequestRequest{
		Repository:        "org/repo",
		EnableReview:      true,
		EnableOverview:    true,
		EnableSuggestions: true,
		Environment:       environment,
		Recipe:            domain.CustomRecipe{},
	})
	require.NoError(t, err)
	require.Equal(t, 1, reviewUseCase.calls)
	require.Equal(t, 1, overviewUseCase.calls)
	require.Same(t, environment, reviewUseCase.lastReq.Environment)
	require.Same(t, environment, overviewUseCase.lastReq.Environment)
}

func TestChangeRequestUseCasePropagatesReviewError(t *testing.T) {
	environment := &changeRequestTestEnv{}
	reviewUseCase := &changeRequestTestReviewUseCase{resultErr: errors.New("review failed")}
	overviewUseCase := &changeRequestTestOverviewUseCase{}
	useCase, err := NewChangeRequestUseCase(reviewUseCase, overviewUseCase, nil)
	require.NoError(t, err)

	_, err = useCase.Execute(context.Background(), ChangeRequestRequest{
		Repository:        "org/repo",
		EnableReview:      true,
		EnableOverview:    false,
		EnableSuggestions: false,
		Environment:       environment,
		Recipe:            domain.CustomRecipe{},
	})
	require.ErrorContains(t, err, "review failed")
}
