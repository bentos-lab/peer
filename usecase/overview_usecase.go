package usecase

import (
	"context"
	"errors"
	"time"

	"bentos-backend/domain"
	"bentos-backend/shared/logger/stdlogger"
	uccontracts "bentos-backend/usecase/contracts"
)

// overviewUseCase is the concrete OverviewUseCase implementation.
type overviewUseCase struct {
	llmOverview LLMOverviewGenerator
	overviewPub OverviewPublisher
	envFactory  uccontracts.CodeEnvironmentFactory
	logger      Logger
}

// NewOverviewUseCase constructs an overview-only usecase.
func NewOverviewUseCase(
	llmOverview LLMOverviewGenerator,
	overviewPub OverviewPublisher,
	envFactory uccontracts.CodeEnvironmentFactory,
	logger Logger,
) (OverviewUseCase, error) {
	if llmOverview == nil || overviewPub == nil || envFactory == nil {
		return nil, errors.New("overview usecase dependencies must not be nil")
	}
	if logger == nil {
		logger = stdlogger.Nop()
	}
	return &overviewUseCase{
		llmOverview: llmOverview,
		overviewPub: overviewPub,
		envFactory:  envFactory,
		logger:      logger,
	}, nil
}

// Execute generates overview content and publishes it to the configured publisher.
func (u *overviewUseCase) Execute(ctx context.Context, request OverviewRequest) (OverviewExecutionResult, error) {
	startedAt := time.Now()
	target := request.Input.Target
	logExecutionStarted(u.logger, "overview", target)

	initializeEnvironmentStartedAt := time.Now()
	environment, err := u.envFactory.New(ctx, domain.CodeEnvironmentInitOptions{
		RepoURL: request.Input.RepoURL,
	})
	if err != nil {
		logStageFailure(u.logger, "overview", "initialize_code_environment", target, initializeEnvironmentStartedAt, err)
		return OverviewExecutionResult{}, err
	}
	defer func() {
		if cleanupErr := environment.Cleanup(ctx); cleanupErr != nil {
			u.logger.Warnf("Failed to cleanup code environment: %v", cleanupErr)
		}
	}()
	u.logger.Infof("Code environment initialized.")
	logStageSuccess(u.logger, "overview", "initialize_code_environment", target, initializeEnvironmentStartedAt)
	u.logger.Debugf("Code environment initialization took %d ms.", time.Since(initializeEnvironmentStartedAt).Milliseconds())

	overviewStartedAt := time.Now()
	overviewResult, err := u.llmOverview.GenerateOverview(ctx, LLMOverviewPayload{
		Input:       request.Input,
		Environment: environment,
	})
	if err != nil {
		logStageFailure(u.logger, "overview", "generate_overview", target, overviewStartedAt, err)
		return OverviewExecutionResult{}, err
	}
	u.logger.Infof("Overview generation completed.")
	logStageSuccess(u.logger, "overview", "generate_overview", target, overviewStartedAt)
	u.logger.Debugf("Overview generation took %d ms.", time.Since(overviewStartedAt).Milliseconds())

	publishStartedAt := time.Now()
	if err := u.overviewPub.PublishOverview(ctx, OverviewPublishRequest{
		Target:   request.Input.Target,
		Overview: overviewResult,
		Metadata: request.Input.Metadata,
	}); err != nil {
		logStageFailure(u.logger, "overview", "publish_overview_result", target, publishStartedAt, err)
		return OverviewExecutionResult{}, err
	}
	u.logger.Infof("Overview publish completed.")
	logStageSuccess(u.logger, "overview", "publish_overview_result", target, publishStartedAt)
	logExecutionCompleted(
		u.logger,
		"overview",
		target,
		startedAt,
		"Overview execution took %d ms.",
		time.Since(startedAt).Milliseconds(),
	)

	return OverviewExecutionResult{Overview: overviewResult}, nil
}
