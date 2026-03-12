package usecase

import (
	"context"
	"testing"

	"bentos-backend/domain"
	uccontracts "bentos-backend/usecase/contracts"
	"github.com/stretchr/testify/require"
)

type changeRequestTestEnvFactory struct {
	env uccontracts.CodeEnvironment
}

func (f *changeRequestTestEnvFactory) New(_ context.Context, _ domain.CodeEnvironmentInitOptions) (uccontracts.CodeEnvironment, error) {
	return f.env, nil
}

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

type changeRequestTestRecipeLoader struct {
	recipe domain.CustomRecipe
}

func (l *changeRequestTestRecipeLoader) Load(_ context.Context, _ uccontracts.CodeEnvironment, _ string) (domain.CustomRecipe, error) {
	return l.recipe, nil
}

type changeRequestTestReviewUseCase struct {
	calls int
}

func (u *changeRequestTestReviewUseCase) Execute(_ context.Context, _ ReviewRequest) (ReviewExecutionResult, error) {
	u.calls++
	return ReviewExecutionResult{}, nil
}

type changeRequestTestOverviewUseCase struct {
	calls int
}

func (u *changeRequestTestOverviewUseCase) Execute(_ context.Context, _ OverviewRequest) (OverviewExecutionResult, error) {
	u.calls++
	return OverviewExecutionResult{}, nil
}

func TestChangeRequestUseCaseSkipsReviewWhenRecipeDisablesIt(t *testing.T) {
	reviewUseCase := &changeRequestTestReviewUseCase{}
	overviewUseCase := &changeRequestTestOverviewUseCase{}
	envFactory := &changeRequestTestEnvFactory{env: &changeRequestTestEnv{}}
	recipeLoader := &changeRequestTestRecipeLoader{
		recipe: domain.CustomRecipe{
			ReviewEnabled: boolPointer(false),
		},
	}
	useCase, err := NewChangeRequestUseCase(reviewUseCase, overviewUseCase, envFactory, recipeLoader, nil)
	require.NoError(t, err)

	_, err = useCase.Execute(context.Background(), ChangeRequestRequest{
		Repository:        "org/repo",
		EnableReview:      true,
		ReviewExplicit:    false,
		EnableOverview:    false,
		OverviewExplicit:  false,
		EnableSuggestions: false,
	})
	require.NoError(t, err)
	require.Equal(t, 0, reviewUseCase.calls)
}

func boolPointer(value bool) *bool {
	return &value
}
