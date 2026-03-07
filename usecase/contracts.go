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

// ChangeRequestRequest is the shared orchestrator input and is platform-neutral.
type ChangeRequestRequest struct {
	Repository          string
	ChangeRequestNumber int
	Title               string
	Description         string
	BaseRef             string
	HeadRef             string
	EnableOverview      bool
	Metadata            map[string]string
}

// ChangeRequestInputProvider loads changed contents for the requested change.
type ChangeRequestInputProvider interface {
	LoadChangeSnapshot(ctx context.Context, request ChangeRequestRequest) (domain.ChangeSnapshot, error)
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
	Review(ctx context.Context, payload LLMReviewPayload) (LLMReviewResult, error)
}

// SuggestionFindingCandidate is one finding candidate passed to grouping/suggest stages.
type SuggestionFindingCandidate struct {
	Key         string
	Finding     domain.Finding
	DiffSnippet string
}

// SuggestionFindingGroup is one LLM-produced finding group.
type SuggestionFindingGroup struct {
	GroupID     string
	Rationale   string
	FindingKeys []string
}

// LLMSuggestionGroupingPayload is the complete grouping prompt payload.
type LLMSuggestionGroupingPayload struct {
	Input        domain.ReviewInput
	Candidates   []SuggestionFindingCandidate
	MaxGroupSize int
}

// LLMSuggestionGroupingResult is normalized grouping output.
type LLMSuggestionGroupingResult struct {
	Groups []SuggestionFindingGroup
}

// LLMSuggestionGrouping groups findings into suggestion batches.
type LLMSuggestionGrouping interface {
	GroupFindings(ctx context.Context, payload LLMSuggestionGroupingPayload) (LLMSuggestionGroupingResult, error)
}

// LLMSuggestedChangePayload is the complete suggested-change prompt payload.
type LLMSuggestedChangePayload struct {
	Input      domain.ReviewInput
	Group      SuggestionFindingGroup
	Candidates []SuggestionFindingCandidate
	GroupDiffs []GroupFileDiffContext
}

// GroupFileDiffContext contains group-scoped diff context for suggestion generation.
type GroupFileDiffContext struct {
	FilePath    string
	DiffSnippet string
}

// FindingSuggestedChange is one suggested change keyed to a finding.
type FindingSuggestedChange struct {
	FindingKey      string
	SuggestedChange domain.SuggestedChange
}

// LLMSuggestedChangeResult is normalized suggested change output.
type LLMSuggestedChangeResult struct {
	Suggestions []FindingSuggestedChange
}

// LLMSuggestedChangeGenerator generates suggested changes per finding group.
type LLMSuggestedChangeGenerator interface {
	GenerateSuggestedChanges(ctx context.Context, payload LLMSuggestedChangePayload) (LLMSuggestedChangeResult, error)
}

// LLMOverviewPayload is the complete overview prompt payload.
type LLMOverviewPayload struct {
	Input domain.OverviewInput
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
	Input domain.ReviewInput
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
