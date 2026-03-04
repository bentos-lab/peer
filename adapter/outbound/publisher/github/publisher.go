package github

import (
	"context"
	"fmt"
	"log"
	"strings"

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
}

// NewPublisher creates a GitHub publisher.
func NewPublisher(client CommentClient) *Publisher {
	return &Publisher{client: client}
}

// Publish posts one anchored review comment per finding and one summary PR comment.
func (p *Publisher) Publish(ctx context.Context, result usecase.ReviewPublishResult) error {
	for _, finding := range result.Findings {
		if err := p.publishFinding(ctx, result.Target, finding); err != nil {
			if githubvcs.IsInvalidAnchorError(err) {
				log.Printf(
					"github publisher skipped invalid anchor repository=%q pull_request_number=%d file=%q start_line=%d end_line=%d error=%v",
					result.Target.Repository,
					result.Target.ChangeRequestNumber,
					finding.FilePath,
					finding.StartLine,
					finding.EndLine,
					err,
				)
				continue
			}
			return err
		}
	}

	summary := strings.TrimSpace(result.Summary)
	if summary == "" {
		summary = "No significant review findings from changed content."
	}
	body := fmt.Sprintf("Review summary\n\n%s", summary)
	if err := p.client.CreateComment(ctx, result.Target.Repository, result.Target.ChangeRequestNumber, body); err != nil {
		return err
	}

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
