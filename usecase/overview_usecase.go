package usecase

import (
	"context"
	"errors"
	"strings"
	"time"

	"bentos-backend/domain"
	"bentos-backend/shared/logger/stdlogger"
)

// overviewUseCase is the concrete OverviewUseCase implementation.
type overviewUseCase struct {
	llmOverview             LLMOverviewGenerator
	issueAlignmentGenerator IssueAlignmentGenerator
	overviewPub             OverviewPublisher
	logger                  Logger
}

// NewOverviewUseCase constructs an overview-only usecase.
func NewOverviewUseCase(
	llmOverview LLMOverviewGenerator,
	issueAlignmentGenerator IssueAlignmentGenerator,
	overviewPub OverviewPublisher,
	logger Logger,
) (OverviewUseCase, error) {
	if llmOverview == nil || issueAlignmentGenerator == nil || overviewPub == nil {
		return nil, errors.New("overview usecase dependencies must not be nil")
	}
	if logger == nil {
		logger = stdlogger.Nop()
	}
	return &overviewUseCase{
		llmOverview:             llmOverview,
		issueAlignmentGenerator: issueAlignmentGenerator,
		overviewPub:             overviewPub,
		logger:                  logger,
	}, nil
}

// Execute generates overview content and publishes it to the configured publisher.
func (u *overviewUseCase) Execute(ctx context.Context, request OverviewRequest) (OverviewExecutionResult, error) {
	startedAt := time.Now()
	target := request.Input.Target
	logExecution(u.logger, "overview", target, "start", startedAt, "")

	if request.Environment == nil {
		return OverviewExecutionResult{}, errors.New("code environment is required")
	}

	overviewStartedAt := time.Now()
	overviewResult, err := u.llmOverview.GenerateOverview(ctx, LLMOverviewPayload{
		Input:         request.Input,
		Environment:   request.Environment,
		ExtraGuidance: strings.TrimSpace(request.Recipe.OverviewGuidance),
	})
	if err != nil {
		logStage(u.logger, "overview", "generate_overview", target, "failure", overviewStartedAt, "%v", err)
		return OverviewExecutionResult{}, err
	}
	logStage(u.logger, "overview", "generate_overview", target, "success", overviewStartedAt, "")

	var issueAlignmentResult *domain.IssueAlignmentResult
	issueAlignmentEnabled := len(request.IssueAlignment.Candidates) > 0
	if request.Recipe.OverviewIssueAlignmentEnabled != nil && !*request.Recipe.OverviewIssueAlignmentEnabled {
		issueAlignmentEnabled = false
	}
	if issueAlignmentEnabled {
		issueAlignmentGuidance := strings.TrimSpace(request.Recipe.OverviewIssueAlignmentGuidance)
		if issueAlignmentGuidance == "" {
			issueAlignmentGuidance = strings.TrimSpace(request.Recipe.OverviewGuidance)
		}
		alignmentStartedAt := time.Now()
		result, err := u.issueAlignmentGenerator.GenerateIssueAlignment(ctx, LLMIssueAlignmentPayload{
			Input:          request.Input,
			IssueAlignment: request.IssueAlignment,
			Environment:    request.Environment,
			ExtraGuidance:  issueAlignmentGuidance,
		})
		if err != nil {
			logStage(u.logger, "overview", "issue_alignment", target, "failure", alignmentStartedAt, "%v", err)
			return OverviewExecutionResult{}, err
		}
		logStage(u.logger, "overview", "issue_alignment", target, "success", alignmentStartedAt, "")
		issueAlignmentResult = &result
	}

	publishStartedAt := time.Now()
	if err := u.overviewPub.PublishOverview(ctx, OverviewPublishRequest{
		Target:         request.Input.Target,
		Overview:       overviewResult,
		IssueAlignment: issueAlignmentResult,
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

	return OverviewExecutionResult{Overview: overviewResult, IssueAlignment: issueAlignmentResult}, nil
}
