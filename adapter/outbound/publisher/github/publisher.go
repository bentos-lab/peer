package github

import (
	"context"
	"fmt"

	"bentos-backend/usecase"
)

// CommentClient posts comments to GitHub PRs.
type CommentClient interface {
	CreateComment(ctx context.Context, repository string, pullRequestNumber int, body string) error
}

// Publisher publishes review comments to GitHub.
type Publisher struct {
	client CommentClient
}

// NewPublisher creates a GitHub publisher.
func NewPublisher(client CommentClient) *Publisher {
	return &Publisher{client: client}
}

// Publish posts each review message as one PR comment.
func (p *Publisher) Publish(ctx context.Context, result usecase.ReviewPublishResult) error {
	for _, message := range result.Messages {
		body := fmt.Sprintf("%s\n\n%s", message.Title, message.Body)
		if err := p.client.CreateComment(ctx, result.Target.Repository, result.Target.ChangeRequestNumber, body); err != nil {
			return err
		}
	}
	return nil
}
