package usecase

import (
	"context"
	"errors"
	"time"

	"bentos-backend/shared/logger/stdlogger"
)

// overviewUseCase is the concrete OverviewUseCase implementation.
type overviewUseCase struct {
	llmOverview LLMOverviewGenerator
	overviewPub OverviewPublisher
	logger      Logger
}

// NewOverviewUseCase constructs an overview-only usecase.
func NewOverviewUseCase(
	llmOverview LLMOverviewGenerator,
	overviewPub OverviewPublisher,
	logger Logger,
) (OverviewUseCase, error) {
	if llmOverview == nil || overviewPub == nil {
		return nil, errors.New("overview usecase dependencies must not be nil")
	}
	if logger == nil {
		logger = stdlogger.Nop()
	}
	return &overviewUseCase{
		llmOverview: llmOverview,
		overviewPub: overviewPub,
		logger:      logger,
	}, nil
}

// Execute generates overview content and publishes it to the configured publisher.
func (u *overviewUseCase) Execute(ctx context.Context, request OverviewRequest) (OverviewExecutionResult, error) {
	startedAt := time.Now()
	u.logger.Infof("Overview execution started.")
	u.logger.Debugf("Overview target is repository %q with change request number %d.", request.Input.Target.Repository, request.Input.Target.ChangeRequestNumber)

	overviewStartedAt := time.Now()
	overviewResult, err := u.llmOverview.GenerateOverview(ctx, LLMOverviewPayload{
		Input: request.Input,
	})
	if err != nil {
		u.logStageFailure(request, "generate_overview", overviewStartedAt, err)
		return OverviewExecutionResult{}, err
	}
	u.logger.Infof("Overview generation completed.")
	u.logger.Debugf("Stage %q finished for repository %q and change request %d.", "generate_overview", request.Input.Target.Repository, request.Input.Target.ChangeRequestNumber)
	u.logger.Debugf("Overview generation took %d ms.", time.Since(overviewStartedAt).Milliseconds())

	publishStartedAt := time.Now()
	if err := u.overviewPub.PublishOverview(ctx, OverviewPublishRequest{
		Target:   request.Input.Target,
		Overview: overviewResult,
		Metadata: request.Input.Metadata,
	}); err != nil {
		u.logger.Errorf("Overview stage failed.")
		u.logger.Debugf("Stage %q failed for repository %q and change request number %d.", "publish_overview_result", request.Input.Target.Repository, request.Input.Target.ChangeRequestNumber)
		u.logger.Debugf("The failed stage ran for %d ms.", time.Since(publishStartedAt).Milliseconds())
		u.logger.Debugf("Failure details: %v.", err)
		return OverviewExecutionResult{}, err
	}
	u.logger.Infof("Overview publish completed.")
	u.logger.Debugf("Stage %q finished for repository %q and change request %d.", "publish_overview_result", request.Input.Target.Repository, request.Input.Target.ChangeRequestNumber)
	u.logger.Debugf("Overview publish took %d ms.", time.Since(publishStartedAt).Milliseconds())

	u.logger.Infof("Overview execution completed.")
	u.logger.Debugf("Overview target was repository %q with change request number %d.", request.Input.Target.Repository, request.Input.Target.ChangeRequestNumber)
	u.logger.Debugf("Overview execution took %d ms.", time.Since(startedAt).Milliseconds())

	return OverviewExecutionResult{Overview: overviewResult}, nil
}

func (u *overviewUseCase) logStageFailure(request OverviewRequest, stage string, startedAt time.Time, err error) {
	u.logger.Errorf("Overview stage failed.")
	u.logger.Debugf("Stage %q failed for repository %q and change request number %d.", stage, request.Input.Target.Repository, request.Input.Target.ChangeRequestNumber)
	u.logger.Debugf("The failed stage ran for %d ms.", time.Since(startedAt).Milliseconds())
	u.logger.Debugf("Failure details: %v.", err)
}
