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
	Repository          string
	RepoURL             string
	ChangeRequestNumber int
	Title               string
	Description         string
	Base                string
	Head                string
	EnableOverview      bool
	EnableSuggestions   bool
	OverviewExplicit    bool
	SuggestionsExplicit bool
	Metadata            map[string]string
}

// RulePackProvider returns hardcoded rule packs.
type RulePackProvider interface {
	CorePack(ctx context.Context) (RulePack, error)
}

// LLMReviewPayload is the complete review prompt payload.
type LLMReviewPayload struct {
	Input         domain.ChangeRequestInput
	RulePack      RulePack
	Environment   uccontracts.CodeEnvironment
	Suggestions   bool
	CustomRuleset string
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
	Input         domain.ChangeRequestInput
	Environment   uccontracts.CodeEnvironment
	ExtraGuidance string
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

// AutogenPayload is the complete autogen prompt payload.
type AutogenPayload struct {
	Input         domain.ChangeRequestInput
	Docs          bool
	Tests         bool
	HeadBranch    string
	Environment   uccontracts.CodeEnvironment
	ExtraGuidance string
}

// AutogenGenerator applies missing tests/docs/comments for the diff.
type AutogenGenerator interface {
	Generate(ctx context.Context, payload AutogenPayload) (string, error)
}

// ReviewPublishResult is output passed to a concrete publisher.
type ReviewPublishResult struct {
	Target         domain.ChangeRequestTarget
	Messages       []domain.ReviewMessage
	Findings       []domain.Finding
	Summary        string
	RecipeWarnings []string
}

// ReviewResultPublisher publishes review results (comment or print).
type ReviewResultPublisher interface {
	Publish(ctx context.Context, result ReviewPublishResult) error
}

// OverviewPublishRequest is output passed to a concrete overview publisher.
type OverviewPublishRequest struct {
	Target         domain.ChangeRequestTarget
	Overview       LLMOverviewResult
	Metadata       map[string]string
	RecipeWarnings []string
}

// OverviewPublisher publishes overview results.
type OverviewPublisher interface {
	PublishOverview(ctx context.Context, req OverviewPublishRequest) error
}

// AutogenPublishRequest is output passed to a concrete autogen publisher.
type AutogenPublishRequest struct {
	Target         domain.ChangeRequestTarget
	Changes        []domain.AutogenChange
	Summary        domain.AutogenSummary
	Publish        bool
	HeadBranch     string
	Metadata       map[string]string
	AgentOutput    string
	Environment    uccontracts.CodeEnvironment
	PushOptions    domain.CodeEnvironmentPushOptions
	RecipeWarnings []string
}

// AutogenPublisher publishes autogen results.
type AutogenPublisher interface {
	PublishAutogen(ctx context.Context, req AutogenPublishRequest) error
}

// ReviewRequest is the review-usecase input.
type ReviewRequest struct {
	Input       domain.ChangeRequestInput
	Suggestions bool
	Recipe      domain.CustomRecipe
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
	Input  domain.ChangeRequestInput
	Recipe domain.CustomRecipe
}

// OverviewExecutionResult is the overview-usecase output.
type OverviewExecutionResult struct {
	Overview LLMOverviewResult
}

// OverviewUseCase defines overview execution behavior.
type OverviewUseCase interface {
	Execute(ctx context.Context, request OverviewRequest) (OverviewExecutionResult, error)
}

// AutogenRequest is the autogen-usecase input.
type AutogenRequest struct {
	Input      domain.ChangeRequestInput
	Docs       bool
	Tests      bool
	Publish    bool
	HeadBranch string
}

// AutogenExecutionResult is the autogen-usecase output.
type AutogenExecutionResult struct {
	Changes     []domain.AutogenChange
	Summary     domain.AutogenSummary
	AgentOutput string
}

// AutogenUseCase defines autogen execution behavior.
type AutogenUseCase interface {
	Execute(ctx context.Context, request AutogenRequest) (AutogenExecutionResult, error)
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
