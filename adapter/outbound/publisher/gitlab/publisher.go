package gitlab

import (
	"context"
	"fmt"
	"time"

	"bentos-backend/shared/logger/stdlogger"
	"bentos-backend/usecase"
)

// NoteClient posts merge request notes to GitLab.
type NoteClient interface {
	CreateMergeRequestNote(ctx context.Context, repository string, mergeRequestNumber int, body string) error
}

// Publisher publishes review notes to GitLab merge requests.
type Publisher struct {
	client NoteClient
	logger usecase.Logger
}

// NewPublisher creates a GitLab publisher.
func NewPublisher(client NoteClient, logger usecase.Logger) *Publisher {
	if logger == nil {
		logger = stdlogger.Nop()
	}
	return &Publisher{client: client, logger: logger}
}

// Publish posts each review message as one MR note.
func (p *Publisher) Publish(ctx context.Context, result usecase.ReviewPublishResult) error {
	startedAt := time.Now()
	p.logger.Infof("Publishing GitLab review result started.")
	p.logger.Debugf("Repository is %q and change request number is %d.", result.Target.Repository, result.Target.ChangeRequestNumber)
	p.logger.Debugf("The publish request includes %d messages.", len(result.Messages))

	for _, message := range result.Messages {
		body := fmt.Sprintf("%s\n\n%s", message.Title, message.Body)
		if err := p.client.CreateMergeRequestNote(ctx, result.Target.Repository, result.Target.ChangeRequestNumber, body); err != nil {
			p.logger.Errorf("Publishing GitLab review result failed.")
			p.logger.Debugf("Repository is %q and change request number is %d.", result.Target.Repository, result.Target.ChangeRequestNumber)
			p.logger.Debugf("The publish operation ran for %d ms before failing.", time.Since(startedAt).Milliseconds())
			p.logger.Debugf("Failure details: %v.", err)
			return err
		}
	}

	p.logger.Infof("Publishing GitLab review result completed.")
	p.logger.Debugf("Repository is %q and change request number is %d.", result.Target.Repository, result.Target.ChangeRequestNumber)
	p.logger.Debugf("The publish operation completed in %d ms and published %d messages.", time.Since(startedAt).Milliseconds(), len(result.Messages))
	return nil
}
