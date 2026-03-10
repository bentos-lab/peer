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
	target := domain.ChangeRequestTarget{
		Repository:          request.Repository,
		ChangeRequestNumber: request.ChangeRequestNumber,
	}
	logExecutionStarted(u.logger, "change request", target)

	input := mapChangeRequestToInput(request)

	var reviewResult ReviewExecutionResult
	var overviewResult OverviewExecutionResult

	if request.EnableOverview {
		overviewStartedAt := time.Now()
		var err error
		overviewResult, err = u.overviewUC.Execute(ctx, OverviewRequest{Input: input})
		if err != nil {
			logStageFailure(u.logger, "change request", "generate_overview", target, overviewStartedAt, err)
			return ChangeRequestExecutionResult{}, err
		}
		logStageSuccess(u.logger, "change request", "generate_overview", target, overviewStartedAt)
	}

	reviewStartedAt := time.Now()
	var err error
	reviewResult, err = u.reviewUseCase.Execute(ctx, ReviewRequest{
		Input:       input,
		Suggestions: request.EnableSuggestions,
	})
	if err != nil {
		logStageFailure(u.logger, "change request", "review_diff", target, reviewStartedAt, err)
		return ChangeRequestExecutionResult{}, err
	}
	logStageSuccess(u.logger, "change request", "review_diff", target, reviewStartedAt)

	logExecutionCompleted(
		u.logger,
		"change request",
		target,
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
		Overview: overviewResult.Overview,
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
