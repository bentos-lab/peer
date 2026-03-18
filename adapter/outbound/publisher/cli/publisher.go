package cli

import (
	"context"
	"fmt"
	"io"

	"bentos-backend/domain"
	"bentos-backend/usecase"
)

// Publisher writes review messages to an output stream.
type Publisher struct {
	writer io.Writer
}

// NewPublisher creates a CLI publisher.
func NewPublisher(writer io.Writer) *Publisher {
	return &Publisher{writer: writer}
}

// Publish prints grouped review messages and summary for CLI.
func (p *Publisher) Publish(_ context.Context, result usecase.ReviewPublishResult) error {
	for _, msg := range result.Messages {
		if msg.Type == domain.ReviewMessageTypeFileGroup {
			if _, err := fmt.Fprintf(p.writer, "[%s] %s\n%s\n\n", msg.Type, msg.Title, msg.Body); err != nil {
				return err
			}
			continue
		}
		if _, err := fmt.Fprintf(p.writer, "[%s] %s\n%s\n", msg.Type, msg.Title, msg.Body); err != nil {
			return err
		}
	}
	return nil
}
