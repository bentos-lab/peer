package usecase

import (
	"context"
	"errors"
	"time"

	"bentos-backend/domain"
	"bentos-backend/shared/logger/stdlogger"
)

// changeRequestUseCase is the concrete ChangeRequestUseCase implementation.
type changeRequestUseCase struct {
	reviewUseCase ReviewUseCase
	overviewUC    OverviewUseCase
	logger        Logger
}

// NewChangeRequestUseCase constructs a platform-agnostic change request orchestrator usecase.
func NewChangeRequestUseCase(
	reviewUseCase ReviewUseCase,
	overviewUC OverviewUseCase,
	logger Logger,
) (ChangeRequestUseCase, error) {
	if reviewUseCase == nil || overviewUC == nil {
		return nil, errors.New("change request usecase dependencies must not be nil")
	}
	if logger == nil {
		logger = stdlogger.Nop()
	}
	return &changeRequestUseCase{
		reviewUseCase: reviewUseCase,
		overviewUC:    overviewUC,
		logger:        logger,
	}, nil
}

// Execute runs the shared change request flow.
func (u *changeRequestUseCase) Execute(ctx context.Context, request ChangeRequestRequest) (ChangeRequestExecutionResult, error) {
	startedAt := time.Now()
	u.logger.Infof("Review execution started.")
	u.logger.Debugf("Review target is repository %q with change request number %d.", request.Repository, request.ChangeRequestNumber)

	reviewInput := mapChangeRequestToReviewInput(request)
	overviewInput := mapChangeRequestToOverviewInput(request)

	var reviewResult ReviewExecutionResult
	var overviewResult OverviewExecutionResult
	var err error

	if request.EnableOverview {
		overviewStartedAt := time.Now()
		overviewResult, err = u.overviewUC.Execute(ctx, OverviewRequest{Input: overviewInput})
		if err != nil {
			u.logStageFailure(request, "generate_overview", overviewStartedAt, err)
			return ChangeRequestExecutionResult{}, err
		}

		reviewStartedAt := time.Now()
		reviewResult, err = u.reviewUseCase.Execute(ctx, ReviewRequest{
			Input:       reviewInput,
			Suggestions: request.EnableSuggestions,
		})
		if err != nil {
			u.logStageFailure(request, "review_diff", reviewStartedAt, err)
			return ChangeRequestExecutionResult{}, err
		}
	} else {
		reviewStartedAt := time.Now()
		reviewResult, err = u.reviewUseCase.Execute(ctx, ReviewRequest{
			Input:       reviewInput,
			Suggestions: request.EnableSuggestions,
		})
		if err != nil {
			u.logStageFailure(request, "review_diff", reviewStartedAt, err)
			return ChangeRequestExecutionResult{}, err
		}
	}

	u.logger.Infof("Review execution completed.")
	u.logger.Debugf("Review target was repository %q with change request number %d.", request.Repository, request.ChangeRequestNumber)
	u.logger.Debugf("Full execution took %d ms and produced %d findings with %d messages.", time.Since(startedAt).Milliseconds(), len(reviewResult.Findings), len(reviewResult.Messages))

	return ChangeRequestExecutionResult{
		Messages: reviewResult.Messages,
		Findings: reviewResult.Findings,
		Summary:  reviewResult.Summary,
		Overview: overviewResult.Overview,
	}, nil
}

func mapChangeRequestToReviewInput(request ChangeRequestRequest) domain.ReviewInput {
	return domain.ReviewInput{
		Target: domain.ReviewTarget{
			Repository:          request.Repository,
			ChangeRequestNumber: request.ChangeRequestNumber,
		},
		RepoURL:     request.RepoURL,
		Base:        request.Base,
		Head:        request.Head,
		Title:       request.Title,
		Description: request.Description,
		Language:    "English",
		Metadata:    request.Metadata,
	}
}

func mapChangeRequestToOverviewInput(request ChangeRequestRequest) domain.OverviewInput {
	return domain.OverviewInput{
		Target: domain.OverviewTarget{
			Repository:          request.Repository,
			ChangeRequestNumber: request.ChangeRequestNumber,
		},
		RepoURL:     request.RepoURL,
		Base:        request.Base,
		Head:        request.Head,
		Title:       request.Title,
		Description: request.Description,
		Language:    "English",
		Metadata:    request.Metadata,
	}
}

func (u *changeRequestUseCase) logStageFailure(request ChangeRequestRequest, stage string, startedAt time.Time, err error) {
	u.logger.Errorf("Review stage failed.")
	u.logger.Debugf("Stage %q failed for repository %q and change request number %d.", stage, request.Repository, request.ChangeRequestNumber)
	u.logger.Debugf("The failed stage ran for %d ms.", time.Since(startedAt).Milliseconds())
	u.logger.Debugf("Failure details: %v.", err)
}
