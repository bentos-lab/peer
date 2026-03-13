package usecase

import (
	"context"
	"errors"
	"strings"
	"time"

	"bentos-backend/domain"
	"bentos-backend/shared/logger/stdlogger"
	uccontracts "bentos-backend/usecase/contracts"
)

// changeRequestUseCase is the concrete ChangeRequestUseCase implementation.
type changeRequestUseCase struct {
	reviewUseCase ReviewUseCase
	overviewUC    OverviewUseCase
	envFactory    uccontracts.CodeEnvironmentFactory
	recipeLoader  CustomRecipeLoader
	logger        Logger
}

// NewChangeRequestUseCase constructs a platform-agnostic change request orchestrator usecase.
func NewChangeRequestUseCase(
	reviewUseCase ReviewUseCase,
	overviewUC OverviewUseCase,
	envFactory uccontracts.CodeEnvironmentFactory,
	recipeLoader CustomRecipeLoader,
	logger Logger,
) (ChangeRequestUseCase, error) {
	if reviewUseCase == nil || overviewUC == nil || envFactory == nil || recipeLoader == nil {
		return nil, errors.New("change request usecase dependencies must not be nil")
	}
	if logger == nil {
		logger = stdlogger.Nop()
	}
	return &changeRequestUseCase{
		reviewUseCase: reviewUseCase,
		overviewUC:    overviewUC,
		envFactory:    envFactory,
		recipeLoader:  recipeLoader,
		logger:        logger,
	}, nil
}

// Execute runs the shared change request flow.
func (u *changeRequestUseCase) Execute(ctx context.Context, request ChangeRequestRequest) (ChangeRequestExecutionResult, error) {
	startedAt := time.Now()
	target := domain.ChangeRequestTarget{
		Repository:          request.Repository,
		ChangeRequestNumber: request.ChangeRequestNumber,
	}
	logExecution(u.logger, "change request", target, "start", startedAt, "")

	input := mapChangeRequestToInput(request)
	recipe, err := u.loadRecipe(ctx, request)
	if err != nil {
		return ChangeRequestExecutionResult{}, err
	}

	effectiveReview := request.EnableReview
	if !request.ReviewExplicit && recipe.ReviewEnabled != nil {
		effectiveReview = *recipe.ReviewEnabled
	}
	effectiveOverview := request.EnableOverview
	if !request.OverviewExplicit && recipe.OverviewEnabled != nil {
		effectiveOverview = *recipe.OverviewEnabled
	}
	effectiveSuggestions := request.EnableSuggestions
	if !request.SuggestionsExplicit && recipe.ReviewSuggestions != nil {
		effectiveSuggestions = *recipe.ReviewSuggestions
	}

	var reviewResult ReviewExecutionResult
	var overviewResult *OverviewExecutionResult

	if effectiveOverview {
		overviewStartedAt := time.Now()
		overviewExecResult, err := u.overviewUC.Execute(ctx, OverviewRequest{
			Input:          input,
			IssueAlignment: request.OverviewIssueAlignment,
			Recipe:         recipe,
		})
		if err != nil {
			logStage(u.logger, "change request", "generate_overview", target, "failure", overviewStartedAt, "%v", err)
			return ChangeRequestExecutionResult{}, err
		}
		logStage(u.logger, "change request", "generate_overview", target, "success", overviewStartedAt, "")
		overviewResult = &overviewExecResult
	}

	if effectiveReview {
		reviewStartedAt := time.Now()
		reviewResult, err = u.reviewUseCase.Execute(ctx, ReviewRequest{
			Input:       input,
			Suggestions: effectiveSuggestions,
			Recipe:      recipe,
		})
		if err != nil {
			logStage(u.logger, "change request", "review_diff", target, "failure", reviewStartedAt, "%v", err)
			return ChangeRequestExecutionResult{}, err
		}
		logStage(u.logger, "change request", "review_diff", target, "success", reviewStartedAt, "")
	} else {
		logStage(u.logger, "change request", "review_diff", target, "skipped", time.Now(), "")
	}

	logExecution(
		u.logger,
		"change request",
		target,
		"complete",
		startedAt,
		"Full execution took %d ms and produced %d findings with %d messages.",
		time.Since(startedAt).Milliseconds(),
		len(reviewResult.Findings),
		len(reviewResult.Messages),
	)

	return ChangeRequestExecutionResult{
		Messages: reviewResult.Messages,
		Findings: reviewResult.Findings,
		Summary:  reviewResult.Summary,
		Overview: func() LLMOverviewResult {
			if overviewResult == nil {
				return LLMOverviewResult{}
			}
			return overviewResult.Overview
		}(),
		IssueAlignment: func() *domain.IssueAlignmentResult {
			if overviewResult == nil {
				return nil
			}
			return overviewResult.IssueAlignment
		}(),
	}, nil
}

func mapChangeRequestToInput(request ChangeRequestRequest) domain.ChangeRequestInput {
	return domain.ChangeRequestInput{
		Target:      domain.ChangeRequestTarget{Repository: request.Repository, ChangeRequestNumber: request.ChangeRequestNumber},
		RepoURL:     request.RepoURL,
		Base:        request.Base,
		Head:        request.Head,
		Title:       request.Title,
		Description: request.Description,
		Language:    "English",
		Metadata:    request.Metadata,
	}
}

func (u *changeRequestUseCase) loadRecipe(ctx context.Context, request ChangeRequestRequest) (domain.CustomRecipe, error) {
	loadStartedAt := time.Now()
	target := domain.ChangeRequestTarget{
		Repository:          request.Repository,
		ChangeRequestNumber: request.ChangeRequestNumber,
	}
	environment, err := u.envFactory.New(ctx, domain.CodeEnvironmentInitOptions{
		RepoURL: request.RepoURL,
	})
	if err != nil {
		logStage(u.logger, "change request", "load_recipe_environment", target, "failure", loadStartedAt, "%v", err)
		return domain.CustomRecipe{}, err
	}
	defer func() {
		if cleanupErr := environment.Cleanup(ctx); cleanupErr != nil {
			u.logger.Warnf("Failed to cleanup code environment: %v", cleanupErr)
		}
	}()

	headRef := strings.TrimSpace(request.Head)
	if headRef == "" {
		headRef = "HEAD"
	}
	recipe, err := u.recipeLoader.Load(ctx, environment, headRef)
	if err != nil {
		logStage(u.logger, "change request", "load_recipe", target, "failure", loadStartedAt, "%v", err)
		return domain.CustomRecipe{}, err
	}
	logStage(u.logger, "change request", "load_recipe", target, "success", loadStartedAt, "")
	return recipe, nil
}
