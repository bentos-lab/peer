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
	logExecution(u.logger, "change request", target, "start", startedAt, "")

	input := mapChangeRequestToInput(request)
	recipe := request.Recipe

	effectiveReview := request.EnableReview
	effectiveOverview := request.EnableOverview
	effectiveSuggestions := request.EnableSuggestions

	if (effectiveReview || effectiveOverview) && request.Environment == nil {
		return ChangeRequestExecutionResult{}, errors.New("code environment is required")
	}

	var reviewResult ReviewExecutionResult
	var overviewResult *OverviewExecutionResult

	if effectiveOverview {
		overviewStartedAt := time.Now()
		overviewExecResult, err := u.overviewUC.Execute(ctx, OverviewRequest{
			Input:          input,
			IssueAlignment: request.OverviewIssueAlignment,
			Recipe:         recipe,
			Environment:    request.Environment,
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
		var reviewErr error
		reviewResult, reviewErr = u.reviewUseCase.Execute(ctx, ReviewRequest{
			Input:       input,
			Suggestions: effectiveSuggestions,
			Recipe:      recipe,
			Environment: request.Environment,
		})
		if reviewErr != nil {
			logStage(u.logger, "change request", "review_diff", target, "failure", reviewStartedAt, "%v", reviewErr)
			return ChangeRequestExecutionResult{}, reviewErr
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
