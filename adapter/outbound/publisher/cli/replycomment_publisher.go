package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/bentos-lab/peer/usecase"
)

// ReplyCommentPublisher prints replycomment output for CLI.
type ReplyCommentPublisher struct {
	writer io.Writer
}

// NewReplyCommentPublisher creates a CLI replycomment publisher.
func NewReplyCommentPublisher(writer io.Writer) *ReplyCommentPublisher {
	return &ReplyCommentPublisher{writer: writer}
}

// Publish prints replycomment content to stdout/stderr.
func (p *ReplyCommentPublisher) Publish(_ context.Context, result usecase.ReplyCommentPublishResult) error {
	if p.writer == nil {
		return fmt.Errorf("replycomment writer is not configured")
	}
	_, err := fmt.Fprintln(p.writer, result.Body)
	return err
}
