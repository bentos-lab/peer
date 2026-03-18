package github

import (
	"context"
	"fmt"

	"github.com/bentos-lab/peer/domain"
	"github.com/bentos-lab/peer/shared/logger/stdlogger"
	"github.com/bentos-lab/peer/usecase"
)

// ReplyCommentClient posts replycomment outputs to GitHub.
type ReplyCommentClient interface {
	CreateComment(ctx context.Context, repository string, pullRequestNumber int, body string) error
	CreateReviewReply(ctx context.Context, repository string, pullRequestNumber int, commentID int64, body string) error
}

// ReplyCommentPublisher publishes replycomment output to GitHub.
type ReplyCommentPublisher struct {
	client ReplyCommentClient
	logger usecase.Logger
}

// NewReplyCommentPublisher creates a GitHub replycomment publisher.
func NewReplyCommentPublisher(client ReplyCommentClient, logger usecase.Logger) *ReplyCommentPublisher {
	if logger == nil {
		logger = stdlogger.Nop()
	}
	return &ReplyCommentPublisher{client: client, logger: logger}
}

// Publish posts the reply to the appropriate GitHub thread.
func (p *ReplyCommentPublisher) Publish(ctx context.Context, result usecase.ReplyCommentPublishResult) error {
	if !result.ShouldPost {
		return nil
	}
	if p.client == nil {
		return fmt.Errorf("replycomment client is not configured")
	}
	if result.Target.ChangeRequestNumber <= 0 {
		return fmt.Errorf("pull request number must be positive")
	}

	body := result.Body
	if warning := usecase.FormatRecipeWarning(result.RecipeWarnings); warning != "" {
		body = fmt.Sprintf("%s\n\n%s", warning, body)
	}

	var err error
	switch result.Kind {
	case domain.CommentKindReview:
		err = p.client.CreateReviewReply(ctx, result.Target.Repository, result.Target.ChangeRequestNumber, result.CommentID, body)
	default:
		err = p.client.CreateComment(ctx, result.Target.Repository, result.Target.ChangeRequestNumber, body)
	}
	if err != nil {
		return err
	}
	return nil
}
