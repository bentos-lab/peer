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

// reviewUseCase is the concrete ReviewUseCase implementation.
type reviewUseCase struct {
	ruleProvider RulePackProvider
	llmReviewer  LLMReviewer
	publisher    ReviewResultPublisher
	envFactory   uccontracts.CodeEnvironmentFactory
	logger       Logger
}

// NewReviewUseCase constructs a review-only usecase.
func NewReviewUseCase(
	ruleProvider RulePackProvider,
	llmReviewer LLMReviewer,
	publisher ReviewResultPublisher,
	envFactory uccontracts.CodeEnvironmentFactory,
	logger Logger,
) (ReviewUseCase, error) {
	if ruleProvider == nil || llmReviewer == nil || publisher == nil || envFactory == nil {
		return nil, errors.New("review usecase dependencies must not be nil")
	}
	if logger == nil {
		logger = stdlogger.Nop()
	}
	u := &reviewUseCase{
		ruleProvider: ruleProvider,
		llmReviewer:  llmReviewer,
		publisher:    publisher,
		envFactory:   envFactory,
		logger:       logger,
	}
	return u, nil
}

// Execute runs the review flow and publishes review messages.
func (u *reviewUseCase) Execute(ctx context.Context, request ReviewRequest) (ReviewExecutionResult, error) {
	startedAt := time.Now()
	target := request.Input.Target
	logExecution(u.logger, "review", target, "start", startedAt, "")

	loadRulePackStartedAt := time.Now()
	pack, err := u.ruleProvider.CorePack(ctx)
	if err != nil {
		logStage(u.logger, "review", "load_rule_pack", target, "failure", loadRulePackStartedAt, "%v", err)
		return ReviewExecutionResult{}, err
	}
	logStage(u.logger, "review", "load_rule_pack", target, "success", loadRulePackStartedAt, "")
	if strings.TrimSpace(request.Recipe.ReviewRuleset) != "" {
		pack.Instructions = []string{strings.TrimSpace(request.Recipe.ReviewRuleset)}
	}
	u.logger.Debugf("Review rule pack loaded with %d instructions.", len(pack.Instructions))

	initializeEnvironmentStartedAt := time.Now()
	environment, err := u.envFactory.New(ctx, domain.CodeEnvironmentInitOptions{
		RepoURL: request.Input.RepoURL,
	})
	if err != nil {
		logStage(u.logger, "review", "initialize_code_environment", target, "failure", initializeEnvironmentStartedAt, "%v", err)
		return ReviewExecutionResult{}, err
	}
	defer func() {
		if cleanupErr := environment.Cleanup(ctx); cleanupErr != nil {
			u.logger.Warnf("Failed to cleanup code environment: %v", cleanupErr)
		}
	}()
	logStage(u.logger, "review", "initialize_code_environment", target, "success", initializeEnvironmentStartedAt, "")

	reviewDiffStartedAt := time.Now()
	llmResult, err := u.llmReviewer.Review(ctx, LLMReviewPayload{
		Input:         request.Input,
		RulePack:      pack,
		Environment:   environment,
		Suggestions:   request.Suggestions,
		CustomRuleset: strings.TrimSpace(request.Recipe.ReviewRuleset),
	})
	if err != nil {
		logStage(u.logger, "review", "review_diff", target, "failure", reviewDiffStartedAt, "%v", err)
		return ReviewExecutionResult{}, err
	}
	logStage(u.logger, "review", "review_diff", target, "success", reviewDiffStartedAt, "")
	u.logger.Debugf("The LLM review produced %d findings.", len(llmResult.Findings))

	publishStartedAt := time.Now()
	messages := BuildMessages(llmResult.Findings, llmResult.Summary)
	publishInput := ReviewPublishResult{
		Target:         request.Input.Target,
		Messages:       messages,
		Findings:       llmResult.Findings,
		Summary:        llmResult.Summary,
		RecipeWarnings: request.Recipe.MissingPaths,
	}
	if err := u.publisher.Publish(ctx, publishInput); err != nil {
		logStage(u.logger, "review", "publish_review_result", target, "failure", publishStartedAt, "%v", err)
		return ReviewExecutionResult{}, err
	}
	logStage(u.logger, "review", "publish_review_result", target, "success", publishStartedAt, "")
	u.logger.Debugf("Published %d review messages.", len(messages))

	logExecution(
		u.logger,
		"review",
		target,
		"complete",
		startedAt,
		"Full execution took %d ms and produced %d findings with %d messages.",
		time.Since(startedAt).Milliseconds(),
		len(llmResult.Findings),
		len(messages),
	)

	return ReviewExecutionResult{
		Messages: messages,
		Findings: llmResult.Findings,
		Summary:  llmResult.Summary,
	}, nil
}
