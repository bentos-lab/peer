package usecase

import (
	"context"
	"errors"
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
	logExecutionStarted(u.logger, "review", target)

	loadRulePackStartedAt := time.Now()
	pack, err := u.ruleProvider.CorePack(ctx)
	if err != nil {
		logStageFailure(u.logger, "review", "load_rule_pack", target, loadRulePackStartedAt, err)
		return ReviewExecutionResult{}, err
	}
	u.logger.Infof("Review rule pack loaded.")
	logStageSuccess(u.logger, "review", "load_rule_pack", target, loadRulePackStartedAt)
	u.logger.Debugf("Loading the rule pack took %d ms and returned %d instructions.", time.Since(loadRulePackStartedAt).Milliseconds(), len(pack.Instructions))

	initializeEnvironmentStartedAt := time.Now()
	environment, err := u.envFactory.New(ctx, domain.CodeEnvironmentInitOptions{
		RepoURL: request.Input.RepoURL,
	})
	if err != nil {
		logStageFailure(u.logger, "review", "initialize_code_environment", target, initializeEnvironmentStartedAt, err)
		return ReviewExecutionResult{}, err
	}
	defer func() {
		if cleanupErr := environment.Cleanup(ctx); cleanupErr != nil {
			u.logger.Warnf("Failed to cleanup code environment: %v", cleanupErr)
		}
	}()
	u.logger.Infof("Code environment initialized.")
	logStageSuccess(u.logger, "review", "initialize_code_environment", target, initializeEnvironmentStartedAt)
	u.logger.Debugf("Code environment initialization took %d ms.", time.Since(initializeEnvironmentStartedAt).Milliseconds())

	reviewDiffStartedAt := time.Now()
	llmResult, err := u.llmReviewer.Review(ctx, LLMReviewPayload{
		Input:       request.Input,
		RulePack:    pack,
		Environment: environment,
		Suggestions: request.Suggestions,
	})
	if err != nil {
		logStageFailure(u.logger, "review", "review_diff", target, reviewDiffStartedAt, err)
		return ReviewExecutionResult{}, err
	}
	u.logger.Infof("LLM review completed.")
	logStageSuccess(u.logger, "review", "review_diff", target, reviewDiffStartedAt)
	u.logger.Debugf("The LLM review produced %d findings.", len(llmResult.Findings))

	publishStartedAt := time.Now()
	messages := BuildMessages(llmResult.Findings, llmResult.Summary)
	publishInput := ReviewPublishResult{
		Target:   request.Input.Target,
		Messages: messages,
		Findings: llmResult.Findings,
		Summary:  llmResult.Summary,
	}
	if err := u.publisher.Publish(ctx, publishInput); err != nil {
		logStageFailure(u.logger, "review", "publish_review_result", target, publishStartedAt, err)
		return ReviewExecutionResult{}, err
	}
	u.logger.Infof("Review result publishing completed.")
	logStageSuccess(u.logger, "review", "publish_review_result", target, publishStartedAt)
	u.logger.Debugf("Publishing review messages took %d ms and sent %d messages.", time.Since(publishStartedAt).Milliseconds(), len(messages))

	logExecutionCompleted(
		u.logger,
		"review",
		target,
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
