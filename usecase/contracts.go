package usecase

import (
	"context"

	"bentos-backend/domain"
	uccontracts "bentos-backend/usecase/contracts"
)

// RulePack defines the active set of review instructions.
type RulePack struct {
	ID           string
	Version      string
	Name         string
	Instructions []string
}

// ChangeRequestRequest is the shared orchestrator input and is platform-neutral.
type ChangeRequestRequest struct {
	Provider            string
	Repository          string
	RepoURL             string
	ChangeRequestNumber int
	Title               string
	Description         string
	Base                string
	Head                string
	Comment             bool
	EnableOverview      bool
	EnableSuggestions   bool
	Metadata            map[string]string
}

// RulePackProvider returns hardcoded rule packs.
type RulePackProvider interface {
	CorePack(ctx context.Context) (RulePack, error)
}

// LLMReviewPayload is the complete review prompt payload.
type LLMReviewPayload struct {
	Input       domain.ReviewInput
	RulePack    RulePack
	Environment uccontracts.CodeEnvironment
	Suggestions bool
}

// LLMReviewResult is normalized LLM output.
type LLMReviewResult struct {
	Findings []domain.Finding
	Summary  string
}

// LLMReviewer reviews changed content and returns findings.
type LLMReviewer interface {
	Review(ctx context.Context, payload LLMReviewPayload) (LLMReviewResult, error)
}

// LLMOverviewPayload is the complete overview prompt payload.
type LLMOverviewPayload struct {
	Input       domain.OverviewInput
	Environment uccontracts.CodeEnvironment
}

// LLMOverviewResult is normalized LLM overview output.
type LLMOverviewResult struct {
	Categories   []domain.OverviewCategoryItem
	Walkthroughs []domain.OverviewWalkthrough
}

// LLMOverviewGenerator creates a high-level overview from changed content.
type LLMOverviewGenerator interface {
	GenerateOverview(ctx context.Context, payload LLMOverviewPayload) (LLMOverviewResult, error)
}

// ReviewPublishResult is output passed to a concrete publisher.
type ReviewPublishResult struct {
	Target   domain.ReviewTarget
	Messages []domain.ReviewMessage
	Findings []domain.Finding
	Summary  string
}

// ReviewResultPublisher publishes review results (comment or print).
type ReviewResultPublisher interface {
	Publish(ctx context.Context, result ReviewPublishResult) error
}

// OverviewPublishRequest is output passed to a concrete overview publisher.
type OverviewPublishRequest struct {
	Target   domain.OverviewTarget
	Overview LLMOverviewResult
	Metadata map[string]string
}

// OverviewPublisher publishes overview results.
type OverviewPublisher interface {
	PublishOverview(ctx context.Context, req OverviewPublishRequest) error
}

// ReviewRequest is the review-usecase input.
type ReviewRequest struct {
	Input       domain.ReviewInput
	Suggestions bool
}

// ReviewExecutionResult is the review-usecase output.
type ReviewExecutionResult struct {
	Messages []domain.ReviewMessage
	Findings []domain.Finding
	Summary  string
}

// ReviewUseCase defines review execution behavior.
type ReviewUseCase interface {
	Execute(ctx context.Context, request ReviewRequest) (ReviewExecutionResult, error)
}

// OverviewRequest is the overview-usecase input.
type OverviewRequest struct {
	Input domain.OverviewInput
}

// OverviewExecutionResult is the overview-usecase output.
type OverviewExecutionResult struct {
	Overview LLMOverviewResult
}

// OverviewUseCase defines overview execution behavior.
type OverviewUseCase interface {
	Execute(ctx context.Context, request OverviewRequest) (OverviewExecutionResult, error)
}

// ChangeRequestExecutionResult is the orchestrator output.
type ChangeRequestExecutionResult struct {
	Messages []domain.ReviewMessage
	Findings []domain.Finding
	Summary  string
	Overview LLMOverviewResult
}

// ChangeRequestUseCase defines shared orchestration behavior.
type ChangeRequestUseCase interface {
	Execute(ctx context.Context, request ChangeRequestRequest) (ChangeRequestExecutionResult, error)
}
