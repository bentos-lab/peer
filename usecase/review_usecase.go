package usecase

import (
	"context"
	"errors"
	"time"
)

// ReviewerUseCase is the concrete ReviewUseCase implementation.
type ReviewerUseCase struct {
	inputProvider ReviewInputProvider
	ruleProvider  RulePackProvider
	llmReviewer   LLMReviewer
	publisher     ReviewResultPublisher
	logger        Logger
}

// NewReviewerUseCase constructs a platform-agnostic review usecase.
func NewReviewerUseCase(
	inputProvider ReviewInputProvider,
	ruleProvider RulePackProvider,
	llmReviewer LLMReviewer,
	publisher ReviewResultPublisher,
	logger Logger,
) (*ReviewerUseCase, error) {
	if inputProvider == nil || ruleProvider == nil || llmReviewer == nil || publisher == nil {
		return nil, errors.New("review usecase dependencies must not be nil")
	}
	if logger == nil {
		logger = NopLogger
	}
	return &ReviewerUseCase{
		inputProvider: inputProvider,
		ruleProvider:  ruleProvider,
		llmReviewer:   llmReviewer,
		publisher:     publisher,
		logger:        logger,
	}, nil
}

// Execute runs the review flow and publishes review messages.
func (u *ReviewerUseCase) Execute(ctx context.Context, request ReviewRequest) (ReviewExecutionResult, error) {
	startedAt := time.Now()
	u.logger.Infof("Review execution started.")
	u.logger.Debugf("Review target is repository %q with change request number %d.", request.Repository, request.ChangeRequestNumber)

	loadInputStartedAt := time.Now()
	input, err := u.inputProvider.LoadReviewInput(ctx, request)
	if err != nil {
		u.logStageFailure(request, "load_review_input", loadInputStartedAt, err)
		return ReviewExecutionResult{}, err
	}
	u.logger.Infof("Review input loaded.")
	u.logger.Debugf("Stage %q finished for repository %q and change request %d.", "load_review_input", request.Repository, request.ChangeRequestNumber)
	u.logger.Debugf("Loading review input took %d ms and found %d changed files.", time.Since(loadInputStartedAt).Milliseconds(), len(input.ChangedFiles))

	loadRulePackStartedAt := time.Now()
	pack, err := u.ruleProvider.CorePack(ctx)
	if err != nil {
		u.logStageFailure(request, "load_rule_pack", loadRulePackStartedAt, err)
		return ReviewExecutionResult{}, err
	}
	u.logger.Infof("Review rule pack loaded.")
	u.logger.Debugf("Stage %q finished for repository %q and change request %d.", "load_rule_pack", request.Repository, request.ChangeRequestNumber)
	u.logger.Debugf("Loading the rule pack took %d ms and returned %d instructions.", time.Since(loadRulePackStartedAt).Milliseconds(), len(pack.Instructions))

	reviewDiffStartedAt := time.Now()
	llmResult, err := u.llmReviewer.ReviewDiff(ctx, LLMReviewPayload{
		Input:    input,
		RulePack: pack,
	})
	if err != nil {
		u.logStageFailure(request, "review_diff", reviewDiffStartedAt, err)
		return ReviewExecutionResult{}, err
	}
	u.logger.Infof("LLM review completed.")
	u.logger.Debugf("Stage %q finished for repository %q and change request %d.", "review_diff", request.Repository, request.ChangeRequestNumber)
	u.logger.Debugf("The LLM review took %d ms and produced %d findings.", time.Since(reviewDiffStartedAt).Milliseconds(), len(llmResult.Findings))

	publishStartedAt := time.Now()
	messages := BuildMessages(llmResult.Findings, llmResult.Summary)
	publishInput := ReviewPublishResult{
		Target:   input.Target,
		Messages: messages,
		Findings: llmResult.Findings,
		Summary:  llmResult.Summary,
	}
	if err := u.publisher.Publish(ctx, publishInput); err != nil {
		u.logStageFailure(request, "publish_review_result", publishStartedAt, err)
		return ReviewExecutionResult{}, err
	}
	u.logger.Infof("Review result publishing completed.")
	u.logger.Debugf("Stage %q finished for repository %q and change request %d.", "publish_review_result", request.Repository, request.ChangeRequestNumber)
	u.logger.Debugf("Publishing review messages took %d ms and sent %d messages.", time.Since(publishStartedAt).Milliseconds(), len(messages))
	u.logger.Infof("Review execution completed.")
	u.logger.Debugf("Review target was repository %q with change request number %d.", request.Repository, request.ChangeRequestNumber)
	u.logger.Debugf("Full execution took %d ms and produced %d findings with %d messages.", time.Since(startedAt).Milliseconds(), len(llmResult.Findings), len(messages))

	return ReviewExecutionResult{
		Messages: messages,
		Findings: llmResult.Findings,
		Summary:  llmResult.Summary,
	}, nil
}

func (u *ReviewerUseCase) logStageFailure(request ReviewRequest, stage string, startedAt time.Time, err error) {
	u.logger.Errorf("Review stage failed.")
	u.logger.Debugf("Stage %q failed for repository %q and change request number %d.", stage, request.Repository, request.ChangeRequestNumber)
	u.logger.Debugf("The failed stage ran for %d ms.", time.Since(startedAt).Milliseconds())
	u.logger.Debugf("Failure details: %v.", err)
}
