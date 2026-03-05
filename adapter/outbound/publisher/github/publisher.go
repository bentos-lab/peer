package github

import (
	"context"
	"fmt"
	"strings"
	"time"

	githubvcs "bentos-backend/adapter/outbound/vcs/github"
	"bentos-backend/domain"
	"bentos-backend/usecase"
)

// CommentClient posts comments to GitHub PRs.
type CommentClient interface {
	CreateComment(ctx context.Context, repository string, pullRequestNumber int, body string) error
	CreateReviewComment(ctx context.Context, repository string, pullRequestNumber int, input githubvcs.CreateReviewCommentInput) error
}

// Publisher publishes review comments to GitHub.
type Publisher struct {
	client CommentClient
	logger usecase.Logger
}

// NewPublisher creates a GitHub publisher.
func NewPublisher(client CommentClient, logger usecase.Logger) *Publisher {
	if logger == nil {
		logger = usecase.NopLogger
	}
	return &Publisher{client: client, logger: logger}
}

// Publish posts one anchored review comment per finding and one summary PR comment.
func (p *Publisher) Publish(ctx context.Context, result usecase.ReviewPublishResult) error {
	startedAt := time.Now()
	p.logger.Infof("Publishing GitHub review result started.")
	p.logger.Debugf("Repository is %q and change request number is %d.", result.Target.Repository, result.Target.ChangeRequestNumber)
	p.logger.Debugf("The publish request includes %d findings.", len(result.Findings))

	for _, finding := range result.Findings {
		if err := p.publishFinding(ctx, result.Target, finding); err != nil {
			if githubvcs.IsInvalidAnchorError(err) {
				p.logger.Infof("Skipped one GitHub review comment because its anchor is invalid.")
				p.logger.Debugf("Repository is %q and change request number is %d.", result.Target.Repository, result.Target.ChangeRequestNumber)
				p.logger.Debugf("The skipped finding used line range %d to %d.", finding.StartLine, finding.EndLine)
				p.logger.Debugf("Failure details: %v.", err)
				continue
			}
			p.logger.Errorf("Publishing GitHub review result failed.")
			p.logger.Debugf("Repository is %q and change request number is %d.", result.Target.Repository, result.Target.ChangeRequestNumber)
			p.logger.Debugf("The publish operation ran for %d ms before failing.", time.Since(startedAt).Milliseconds())
			p.logger.Debugf("Failure details: %v.", err)
			return err
		}
	}

	summary := strings.TrimSpace(result.Summary)
	if summary == "" {
		summary = "No significant review findings from changed content."
	}
	body := fmt.Sprintf("Review summary\n\n%s", summary)
	if err := p.client.CreateComment(ctx, result.Target.Repository, result.Target.ChangeRequestNumber, body); err != nil {
		p.logger.Errorf("Publishing GitHub review summary failed.")
		p.logger.Debugf("Repository is %q and change request number is %d.", result.Target.Repository, result.Target.ChangeRequestNumber)
		p.logger.Debugf("The publish operation ran for %d ms before failing.", time.Since(startedAt).Milliseconds())
		p.logger.Debugf("Failure details: %v.", err)
		return err
	}

	p.logger.Infof("Publishing GitHub review result completed.")
	p.logger.Debugf("Repository is %q and change request number is %d.", result.Target.Repository, result.Target.ChangeRequestNumber)
	p.logger.Debugf("The publish operation completed in %d ms.", time.Since(startedAt).Milliseconds())
	p.logger.Debugf("Published %d summary message and processed %d findings.", 1, len(result.Findings))

	return nil
}

func (p *Publisher) publishFinding(ctx context.Context, target domain.ReviewTarget, finding domain.Finding) error {
	filePath := strings.TrimSpace(finding.FilePath)
	if filePath == "" {
		return &githubvcs.InvalidAnchorError{Message: "invalid review comment anchor: file path is empty"}
	}
	if finding.StartLine <= 0 || finding.EndLine <= 0 || finding.StartLine > finding.EndLine {
		return &githubvcs.InvalidAnchorError{
			Message: fmt.Sprintf("invalid review comment anchor: invalid range startLine=%d endLine=%d", finding.StartLine, finding.EndLine),
		}
	}

	commentBody := fmt.Sprintf("[%s] %s\n\n%s", finding.Severity, finding.Title, finding.Detail)
	if strings.TrimSpace(finding.Suggestion) != "" {
		commentBody = fmt.Sprintf("%s\n\nSuggested change: %s", commentBody, finding.Suggestion)
	}

	return p.client.CreateReviewComment(ctx, target.Repository, target.ChangeRequestNumber, githubvcs.CreateReviewCommentInput{
		Body:      commentBody,
		Path:      filePath,
		StartLine: finding.StartLine,
		EndLine:   finding.EndLine,
	})
}
