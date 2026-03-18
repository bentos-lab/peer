package router

import (
	"context"
	"fmt"

	"github.com/bentos-lab/peer/usecase"
)

// ReplyCommentPublisher routes replycomment output to comment or print publishers.
type ReplyCommentPublisher struct {
	Print   usecase.ReplyCommentPublisher
	Comment usecase.ReplyCommentPublisher
}

// NewReplyCommentPublisher creates a replycomment publisher router.
func NewReplyCommentPublisher(printPublisher usecase.ReplyCommentPublisher, commentPublisher usecase.ReplyCommentPublisher) *ReplyCommentPublisher {
	return &ReplyCommentPublisher{Print: printPublisher, Comment: commentPublisher}
}

// Publish routes to the comment publisher when ShouldPost is true; otherwise print.
func (p *ReplyCommentPublisher) Publish(ctx context.Context, result usecase.ReplyCommentPublishResult) error {
	if result.ShouldPost {
		if p.Comment != nil {
			return p.Comment.Publish(ctx, result)
		}
		return fmt.Errorf("replycomment comment publisher is not configured")
	}
	if p.Print != nil {
		return p.Print.Publish(ctx, result)
	}
	return fmt.Errorf("replycomment print publisher is not configured")
}
