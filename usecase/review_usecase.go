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
	u.logger.Infof("Review execution started.")
	u.logger.Debugf("Review target is repository %q with change request number %d.", request.Input.Target.Repository, request.Input.Target.ChangeRequestNumber)

	loadRulePackStartedAt := time.Now()
	pack, err := u.ruleProvider.CorePack(ctx)
	if err != nil {
		u.logStageFailure(request, "load_rule_pack", loadRulePackStartedAt, err)
		return ReviewExecutionResult{}, err
	}
	u.logger.Infof("Review rule pack loaded.")
	u.logger.Debugf("Stage %q finished for repository %q and change request %d.", "load_rule_pack", request.Input.Target.Repository, request.Input.Target.ChangeRequestNumber)
	u.logger.Debugf("Loading the rule pack took %d ms and returned %d instructions.", time.Since(loadRulePackStartedAt).Milliseconds(), len(pack.Instructions))

	initializeEnvironmentStartedAt := time.Now()
	environment, err := u.envFactory.New(ctx, domain.CodeEnvironmentInitOptions{
		RepoURL: request.Input.RepoURL,
	})
	if err != nil {
		u.logStageFailure(request, "initialize_code_environment", initializeEnvironmentStartedAt, err)
		return ReviewExecutionResult{}, err
	}
	u.logger.Infof("Code environment initialized.")
	u.logger.Debugf("Stage %q finished for repository %q and change request %d.", "initialize_code_environment", request.Input.Target.Repository, request.Input.Target.ChangeRequestNumber)
	u.logger.Debugf("Code environment initialization took %d ms.", time.Since(initializeEnvironmentStartedAt).Milliseconds())

	reviewDiffStartedAt := time.Now()
	llmResult, err := u.llmReviewer.Review(ctx, LLMReviewPayload{
		Input:       request.Input,
		RulePack:    pack,
		Environment: environment,
		Suggestions: request.Suggestions,
	})
	if err != nil {
		u.logStageFailure(request, "review_diff", reviewDiffStartedAt, err)
		return ReviewExecutionResult{}, err
	}
	u.logger.Infof("LLM review completed.")
	u.logger.Debugf("Stage %q finished for repository %q and change request %d.", "review_diff", request.Input.Target.Repository, request.Input.Target.ChangeRequestNumber)
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
		u.logStageFailure(request, "publish_review_result", publishStartedAt, err)
		return ReviewExecutionResult{}, err
	}
	u.logger.Infof("Review result publishing completed.")
	u.logger.Debugf("Stage %q finished for repository %q and change request %d.", "publish_review_result", request.Input.Target.Repository, request.Input.Target.ChangeRequestNumber)
	u.logger.Debugf("Publishing review messages took %d ms and sent %d messages.", time.Since(publishStartedAt).Milliseconds(), len(messages))

	u.logger.Infof("Review execution completed.")
	u.logger.Debugf("Review target was repository %q with change request number %d.", request.Input.Target.Repository, request.Input.Target.ChangeRequestNumber)
	u.logger.Debugf("Full execution took %d ms and produced %d findings with %d messages.", time.Since(startedAt).Milliseconds(), len(llmResult.Findings), len(messages))

	return ReviewExecutionResult{
		Messages: messages,
		Findings: llmResult.Findings,
		Summary:  llmResult.Summary,
	}, nil
}

func (u *reviewUseCase) logStageFailure(request ReviewRequest, stage string, startedAt time.Time, err error) {
	u.logger.Errorf("Review stage failed.")
	u.logger.Debugf("Stage %q failed for repository %q and change request number %d.", stage, request.Input.Target.Repository, request.Input.Target.ChangeRequestNumber)
	u.logger.Debugf("The failed stage ran for %d ms.", time.Since(startedAt).Milliseconds())
	u.logger.Debugf("Failure details: %v.", err)
}
