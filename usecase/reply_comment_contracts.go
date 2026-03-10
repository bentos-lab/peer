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
	Input       domain.ChangeRequestInput
	Thread      domain.CommentThread
	Question    string
	Environment uccontracts.CodeEnvironment
}

// ReplyCommentAnswerer generates an answer from code context.
type ReplyCommentAnswerer interface {
	Answer(ctx context.Context, payload ReplyCommentAnswerPayload) (string, error)
}

// SanitizedQuestion captures the sanitized question output.
type SanitizedQuestion struct {
	Status            domain.QuestionSafetyStatusEnum
	SanitizedQuestion string
	RefusalMessage    string
}

// ReplyCommentSanitizer cleans and classifies the question.
type ReplyCommentSanitizer interface {
	Sanitize(ctx context.Context, question string) (SanitizedQuestion, error)
}

// ReplyCommentPublishResult is the payload for publishing a reply.
type ReplyCommentPublishResult struct {
	Target     domain.ChangeRequestTarget
	CommentID  int64
	Kind       domain.CommentKindEnum
	Body       string
	ShouldPost bool
}

// ReplyCommentPublisher publishes replycomment answers.
type ReplyCommentPublisher interface {
	Publish(ctx context.Context, result ReplyCommentPublishResult) error
}
