package usecase

import (
	"context"
	"errors"
)

// ReviewerUseCase is the concrete ReviewUseCase implementation.
type ReviewerUseCase struct {
	inputProvider ReviewInputProvider
	ruleProvider  RulePackProvider
	llmReviewer   LLMReviewer
	publisher     ReviewResultPublisher
}

// NewReviewerUseCase constructs a platform-agnostic review usecase.
func NewReviewerUseCase(
	inputProvider ReviewInputProvider,
	ruleProvider RulePackProvider,
	llmReviewer LLMReviewer,
	publisher ReviewResultPublisher,
) (*ReviewerUseCase, error) {
	if inputProvider == nil || ruleProvider == nil || llmReviewer == nil || publisher == nil {
		return nil, errors.New("review usecase dependencies must not be nil")
	}
	return &ReviewerUseCase{
		inputProvider: inputProvider,
		ruleProvider:  ruleProvider,
		llmReviewer:   llmReviewer,
		publisher:     publisher,
	}, nil
}

// Execute runs the review flow and publishes review messages.
func (u *ReviewerUseCase) Execute(ctx context.Context, request ReviewRequest) (ReviewExecutionResult, error) {
	input, err := u.inputProvider.LoadReviewInput(ctx, request)
	if err != nil {
		return ReviewExecutionResult{}, err
	}

	pack, err := u.ruleProvider.CorePack(ctx)
	if err != nil {
		return ReviewExecutionResult{}, err
	}

	llmResult, err := u.llmReviewer.ReviewDiff(ctx, LLMReviewPayload{
		Input:    input,
		RulePack: pack,
	})
	if err != nil {
		return ReviewExecutionResult{}, err
	}

	messages := BuildMessages(llmResult.Findings, llmResult.Summary)
	publishInput := ReviewPublishResult{
		ReviewID: request.ReviewID,
		Target:   input.Target,
		Messages: messages,
		Findings: llmResult.Findings,
		Summary:  llmResult.Summary,
	}
	if err := u.publisher.Publish(ctx, publishInput); err != nil {
		return ReviewExecutionResult{}, err
	}

	return ReviewExecutionResult{
		Messages: messages,
		Findings: llmResult.Findings,
		Summary:  llmResult.Summary,
	}, nil
}
