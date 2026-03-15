package usecase

import (
	"context"

	"bentos-backend/domain"
	uccontracts "bentos-backend/usecase/contracts"
)

// ReplyCommentRequest is the input to the reply comment usecase.
type ReplyCommentRequest struct {
	Repository          string
	RepoURL             string
	ChangeRequestNumber int
	Title               string
	Description         string
	Base                string
	Head                string
	CommentID           int64
	CommentKind         domain.CommentKindEnum
	Question            string
	Thread              domain.CommentThread
	Publish             bool
	Metadata            map[string]string
	Environment         uccontracts.CodeEnvironment
	Recipe              domain.CustomRecipe
}

// ReplyCommentResult is the usecase output.
type ReplyCommentResult struct {
	Answer string
}

// ReplyCommentUseCase defines the reply comment execution flow.
type ReplyCommentUseCase interface {
	Execute(ctx context.Context, request ReplyCommentRequest) (ReplyCommentResult, error)
}

// ReplyCommentAnswerPayload is the payload for the coding agent adapter.
type ReplyCommentAnswerPayload struct {
	Input         domain.ChangeRequestInput
	Thread        domain.CommentThread
	Question      string
	Environment   uccontracts.CodeEnvironment
	ExtraGuidance string
}

// ReplyCommentAnswerer generates an answer from code context.
type ReplyCommentAnswerer interface {
	Answer(ctx context.Context, payload ReplyCommentAnswerPayload) (string, error)
}

// SanitizedPrompt captures the sanitized prompt output.
type SanitizedPrompt struct {
	Status          domain.PromptSafetyStatusEnum
	SanitizedPrompt string
	RefusalMessage  string
}

// SafetySanitizer cleans and classifies prompts.
type SafetySanitizer interface {
	Sanitize(ctx context.Context, prompt string) (SanitizedPrompt, error)
}

// ReplyCommentPublishResult is the payload for publishing a reply.
type ReplyCommentPublishResult struct {
	Target         domain.ChangeRequestTarget
	CommentID      int64
	Kind           domain.CommentKindEnum
	Body           string
	ShouldPost     bool
	RecipeWarnings []string
}

// ReplyCommentPublisher publishes replycomment answers.
type ReplyCommentPublisher interface {
	Publish(ctx context.Context, result ReplyCommentPublishResult) error
}
