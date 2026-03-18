package usecase

import (
	"context"

	"bentos-backend/domain"
)

// RulePack defines the active set of review instructions.
type RulePack struct {
	ID           string
	Version      string
	Name         string
	Instructions []string
}

// ReviewRequest is the usecase input and is platform-neutral.
type ReviewRequest struct {
	ReviewID            string
	Repository          string
	ChangeRequestNumber int
	Title               string
	Description         string
	BaseRef             string
	HeadRef             string
	Metadata            map[string]string
}

// ReviewInputProvider loads changed contents for the requested change.
type ReviewInputProvider interface {
	LoadReviewInput(ctx context.Context, request ReviewRequest) (domain.ReviewInput, error)
}

// RulePackProvider returns hardcoded rule packs.
type RulePackProvider interface {
	CorePack(ctx context.Context) (RulePack, error)
}

// LLMReviewPayload is the complete review prompt payload.
type LLMReviewPayload struct {
	Input    domain.ReviewInput
	RulePack RulePack
}

// LLMReviewResult is normalized LLM output.
type LLMReviewResult struct {
	Findings []domain.Finding
	Summary  string
}

// LLMReviewer reviews changed content and returns findings.
type LLMReviewer interface {
	ReviewDiff(ctx context.Context, payload LLMReviewPayload) (LLMReviewResult, error)
}

// ReviewPublishResult is output passed to a concrete publisher.
type ReviewPublishResult struct {
	ReviewID string
	Target   domain.ReviewTarget
	Messages []domain.ReviewMessage
}

// ReviewResultPublisher publishes review results (comment or print).
type ReviewResultPublisher interface {
	Publish(ctx context.Context, result ReviewPublishResult) error
}

// ReviewUseCase defines review execution behavior.
type ReviewUseCase interface {
	Execute(ctx context.Context, request ReviewRequest) (ReviewExecutionResult, error)
}

// ReviewExecutionResult is the final usecase output.
type ReviewExecutionResult struct {
	Messages []domain.ReviewMessage
	Findings []domain.Finding
	Summary  string
}
