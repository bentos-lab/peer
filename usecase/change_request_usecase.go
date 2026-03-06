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
	inputProvider ChangeRequestInputProvider
	reviewUseCase ReviewUseCase
	overviewUC    OverviewUseCase
	logger        Logger
}

// NewChangeRequestUseCase constructs a platform-agnostic change request orchestrator usecase.
func NewChangeRequestUseCase(
	inputProvider ChangeRequestInputProvider,
	reviewUseCase ReviewUseCase,
	overviewUC OverviewUseCase,
	logger Logger,
) (ChangeRequestUseCase, error) {
	if inputProvider == nil || reviewUseCase == nil || overviewUC == nil {
		return nil, errors.New("change request usecase dependencies must not be nil")
	}
	if logger == nil {
		logger = stdlogger.Nop()
	}
	return &changeRequestUseCase{
		inputProvider: inputProvider,
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

	loadInputStartedAt := time.Now()
	snapshot, err := u.inputProvider.LoadChangeSnapshot(ctx, request)
	if err != nil {
		u.logStageFailure(request, "load_review_input", loadInputStartedAt, err)
		return ChangeRequestExecutionResult{}, err
	}
	u.logger.Infof("Review input loaded.")
	u.logger.Debugf("Stage %q finished for repository %q and change request %d.", "load_review_input", request.Repository, request.ChangeRequestNumber)
	u.logger.Debugf("Loading review input took %d ms and found %d changed files.", time.Since(loadInputStartedAt).Milliseconds(), len(snapshot.ChangedFiles))

	reviewInput := mapChangeSnapshotToReviewInput(snapshot)
	overviewInput := mapChangeSnapshotToOverviewInput(snapshot)

	var reviewResult ReviewExecutionResult
	var overviewResult OverviewExecutionResult

	if request.EnableOverview {
		overviewStartedAt := time.Now()
		overviewResult, err = u.overviewUC.Execute(ctx, OverviewRequest{Input: overviewInput})
		if err != nil {
			u.logStageFailure(request, "generate_overview", overviewStartedAt, err)
			return ChangeRequestExecutionResult{}, err
		}

		reviewStartedAt := time.Now()
		reviewResult, err = u.reviewUseCase.Execute(ctx, ReviewRequest{Input: reviewInput})
		if err != nil {
			u.logStageFailure(request, "review_diff", reviewStartedAt, err)
			return ChangeRequestExecutionResult{}, err
		}
	} else {
		reviewStartedAt := time.Now()
		reviewResult, err = u.reviewUseCase.Execute(ctx, ReviewRequest{Input: reviewInput})
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

func mapChangeSnapshotToReviewInput(snapshot domain.ChangeSnapshot) domain.ReviewInput {
	return domain.ReviewInput{
		Target: domain.ReviewTarget{
			Repository:          snapshot.Context.Repository,
			ChangeRequestNumber: snapshot.Context.ChangeRequestNumber,
		},
		Title:         snapshot.Context.Title,
		Description:   snapshot.Context.Description,
		ChangedFiles:  snapshot.ChangedFiles,
		Language:      snapshot.Language,
		Metadata:      snapshot.Context.Metadata,
		SourceContext: snapshot.SourceContext,
	}
}

func mapChangeSnapshotToOverviewInput(snapshot domain.ChangeSnapshot) domain.OverviewInput {
	return domain.OverviewInput{
		Target: domain.OverviewTarget{
			Repository:          snapshot.Context.Repository,
			ChangeRequestNumber: snapshot.Context.ChangeRequestNumber,
		},
		Title:         snapshot.Context.Title,
		Description:   snapshot.Context.Description,
		ChangedFiles:  snapshot.ChangedFiles,
		Language:      snapshot.Language,
		Metadata:      snapshot.Context.Metadata,
		SourceContext: snapshot.SourceContext,
	}
}

func (u *changeRequestUseCase) logStageFailure(request ChangeRequestRequest, stage string, startedAt time.Time, err error) {
	u.logger.Errorf("Review stage failed.")
	u.logger.Debugf("Stage %q failed for repository %q and change request number %d.", stage, request.Repository, request.ChangeRequestNumber)
	u.logger.Debugf("The failed stage ran for %d ms.", time.Since(startedAt).Milliseconds())
	u.logger.Debugf("Failure details: %v.", err)
}
