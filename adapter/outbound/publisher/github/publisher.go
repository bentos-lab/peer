package github

import (
	"context"
	"fmt"
	"strings"
	"time"

	githubvcs "bentos-backend/adapter/outbound/vcs/github"
	"bentos-backend/domain"
	"bentos-backend/shared/logger/stdlogger"
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
		logger = stdlogger.Nop()
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
		input, err := p.buildReviewCommentInput(finding)
		if err != nil {
			p.logFindingPayload("skipped_invalid_anchor", result.Target, finding, buildFindingCommentBody(finding))
			p.logger.Warnf("Skipped one GitHub review comment because its anchor is invalid.")
			p.logger.Debugf("Repository is %q and change request number is %d.", result.Target.Repository, result.Target.ChangeRequestNumber)
			p.logger.Debugf("The skipped finding used line range %d to %d.", finding.StartLine, finding.EndLine)
			p.logger.Debugf("Failure details: %v.", err)
			continue
		}

		if err := p.client.CreateReviewComment(ctx, result.Target.Repository, result.Target.ChangeRequestNumber, input); err != nil {
			if githubvcs.IsInvalidAnchorError(err) {
				p.logFindingPayload("skipped_invalid_anchor", result.Target, finding, input.Body)
				p.logger.Warnf("Skipped one GitHub review comment because its anchor is invalid.")
				p.logger.Debugf("Repository is %q and change request number is %d.", result.Target.Repository, result.Target.ChangeRequestNumber)
				p.logger.Debugf("The skipped finding used line range %d to %d.", finding.StartLine, finding.EndLine)
				p.logger.Debugf("Failure details: %v.", err)
				continue
			}
			p.logFindingPayload("failed", result.Target, finding, input.Body)
			p.logger.Errorf("Publishing GitHub review result failed.")
			p.logger.Debugf("Repository is %q and change request number is %d.", result.Target.Repository, result.Target.ChangeRequestNumber)
			p.logger.Debugf("The publish operation ran for %d ms before failing.", time.Since(startedAt).Milliseconds())
			p.logger.Debugf("Failure details: %v.", err)
			return err
		}

		p.logFindingPayload("success", result.Target, finding, input.Body)
	}

	summary := strings.TrimSpace(result.Summary)
	if summary == "" {
		summary = "No significant review findings from changed content."
	}
	body := fmt.Sprintf("Review summary\n\n%s", summary)
	if err := p.client.CreateComment(ctx, result.Target.Repository, result.Target.ChangeRequestNumber, body); err != nil {
		p.logSummaryPayload("failed", result.Target, body)
		p.logger.Errorf("Publishing GitHub review summary failed.")
		p.logger.Debugf("Repository is %q and change request number is %d.", result.Target.Repository, result.Target.ChangeRequestNumber)
		p.logger.Debugf("The publish operation ran for %d ms before failing.", time.Since(startedAt).Milliseconds())
		p.logger.Debugf("Failure details: %v.", err)
		return err
	}
	p.logSummaryPayload("success", result.Target, body)

	p.logger.Infof("Publishing GitHub review result completed.")
	p.logger.Debugf("Repository is %q and change request number is %d.", result.Target.Repository, result.Target.ChangeRequestNumber)
	p.logger.Debugf("The publish operation completed in %d ms.", time.Since(startedAt).Milliseconds())
	p.logger.Debugf("Published %d summary message and processed %d findings.", 1, len(result.Findings))

	return nil
}

func (p *Publisher) buildReviewCommentInput(finding domain.Finding) (githubvcs.CreateReviewCommentInput, error) {
	filePath := strings.TrimSpace(finding.FilePath)
	if filePath == "" {
		return githubvcs.CreateReviewCommentInput{}, &githubvcs.InvalidAnchorError{Message: "invalid review comment anchor: file path is empty"}
	}
	if finding.StartLine <= 0 || finding.EndLine <= 0 || finding.StartLine > finding.EndLine {
		return githubvcs.CreateReviewCommentInput{}, &githubvcs.InvalidAnchorError{
			Message: fmt.Sprintf("invalid review comment anchor: invalid range startLine=%d endLine=%d", finding.StartLine, finding.EndLine),
		}
	}

	return githubvcs.CreateReviewCommentInput{
		Body:      buildFindingCommentBody(finding),
		Path:      filePath,
		StartLine: finding.StartLine,
		EndLine:   finding.EndLine,
	}, nil
}

func buildFindingCommentBody(finding domain.Finding) string {
	commentBody := fmt.Sprintf("[%s] %s\n\n%s", finding.Severity, finding.Title, finding.Detail)
	if block, ok := renderSuggestedChangeBlock(finding); ok {
		return fmt.Sprintf("%s\n\n%s", commentBody, block)
	}
	if strings.TrimSpace(finding.Suggestion) != "" {
		return fmt.Sprintf("%s\n\nSuggested change: %s", commentBody, finding.Suggestion)
	}
	return commentBody
}

func (p *Publisher) logFindingPayload(state string, target domain.ReviewTarget, finding domain.Finding, commentBody string) {
	p.logger.Debugf("GitHub review comment metadata state=%q repo=%q pr=%d file=%q startLine=%d endLine=%d severity=%q title=%q.",
		state, target.Repository, target.ChangeRequestNumber, finding.FilePath, finding.StartLine, finding.EndLine, finding.Severity, finding.Title)

	suggestedChangeKind := ""
	suggestedChangeReason := ""
	suggestedChangeReplacement := ""
	if finding.SuggestedChange != nil {
		suggestedChangeKind = string(finding.SuggestedChange.Kind)
		suggestedChangeReason = finding.SuggestedChange.Reason
		suggestedChangeReplacement = finding.SuggestedChange.Replacement
	}

	p.logger.Tracef("GitHub review comment content state=%q detail=%q suggestion=%q suggestedChangeKind=%q suggestedChangeReason=%q suggestedChangeReplacement=%q body=%q.",
		state, finding.Detail, finding.Suggestion, suggestedChangeKind, suggestedChangeReason, suggestedChangeReplacement, commentBody)
}

func (p *Publisher) logSummaryPayload(state string, target domain.ReviewTarget, body string) {
	p.logger.Debugf("GitHub review summary metadata state=%q repo=%q pr=%d.", state, target.Repository, target.ChangeRequestNumber)
	p.logger.Tracef("GitHub review summary content state=%q body=%q.", state, body)
}

func renderSuggestedChangeBlock(finding domain.Finding) (string, bool) {
	if finding.SuggestedChange == nil {
		return "", false
	}

	switch finding.SuggestedChange.Kind {
	case domain.SuggestedChangeKindReplace:
		replacement := strings.TrimSpace(finding.SuggestedChange.Replacement)
		if replacement == "" {
			return "", false
		}
		return fmt.Sprintf("```suggestion\n%s\n```", finding.SuggestedChange.Replacement), true
	case domain.SuggestedChangeKindDelete:
		if finding.SuggestedChange.Replacement != "" {
			return "", false
		}
		return "```suggestion\n\n```", true
	default:
		return "", false
	}
}
