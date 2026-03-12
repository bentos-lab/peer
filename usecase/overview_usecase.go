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
	logExecution(u.logger, "overview", target, "start", startedAt, "")

	initializeEnvironmentStartedAt := time.Now()
	environment, err := u.envFactory.New(ctx, domain.CodeEnvironmentInitOptions{
		RepoURL: request.Input.RepoURL,
	})
	if err != nil {
		logStage(u.logger, "overview", "initialize_code_environment", target, "failure", initializeEnvironmentStartedAt, "%v", err)
		return OverviewExecutionResult{}, err
	}
	defer func() {
		if cleanupErr := environment.Cleanup(ctx); cleanupErr != nil {
			u.logger.Warnf("Failed to cleanup code environment: %v", cleanupErr)
		}
	}()
	logStage(u.logger, "overview", "initialize_code_environment", target, "success", initializeEnvironmentStartedAt, "")

	overviewStartedAt := time.Now()
	overviewResult, err := u.llmOverview.GenerateOverview(ctx, LLMOverviewPayload{
		Input:         request.Input,
		Environment:   environment,
		ExtraGuidance: strings.TrimSpace(request.Recipe.ReviewOverviewGuidance),
	})
	if err != nil {
		logStage(u.logger, "overview", "generate_overview", target, "failure", overviewStartedAt, "%v", err)
		return OverviewExecutionResult{}, err
	}
	logStage(u.logger, "overview", "generate_overview", target, "success", overviewStartedAt, "")

	publishStartedAt := time.Now()
	if err := u.overviewPub.PublishOverview(ctx, OverviewPublishRequest{
		Target:         request.Input.Target,
		Overview:       overviewResult,
		Metadata:       request.Input.Metadata,
		RecipeWarnings: request.Recipe.MissingPaths,
	}); err != nil {
		logStage(u.logger, "overview", "publish_overview_result", target, "failure", publishStartedAt, "%v", err)
		return OverviewExecutionResult{}, err
	}
	logStage(u.logger, "overview", "publish_overview_result", target, "success", publishStartedAt, "")
	logExecution(
		u.logger,
		"overview",
		target,
		"complete",
		startedAt,
		"Overview execution took %d ms.",
		time.Since(startedAt).Milliseconds(),
	)

	return OverviewExecutionResult{Overview: overviewResult}, nil
}
