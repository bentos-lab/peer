package gitlab

import (
	"context"
	"fmt"

	"bentos-backend/usecase"
)

// NoteClient posts merge request notes to GitLab.
type NoteClient interface {
	CreateMergeRequestNote(ctx context.Context, repository string, mergeRequestNumber int, body string) error
}

// Publisher publishes review notes to GitLab merge requests.
type Publisher struct {
	client NoteClient
}

// NewPublisher creates a GitLab publisher.
func NewPublisher(client NoteClient) *Publisher {
	return &Publisher{client: client}
}

// Publish posts each review message as one MR note.
func (p *Publisher) Publish(ctx context.Context, result usecase.ReviewPublishResult) error {
	for _, message := range result.Messages {
		body := fmt.Sprintf("%s\n\n%s", message.Title, message.Body)
		if err := p.client.CreateMergeRequestNote(ctx, result.Target.Repository, result.Target.ChangeRequestNumber, body); err != nil {
			return err
		}
	}
	return nil
}
