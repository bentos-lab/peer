package github

import (
	"context"
	"fmt"
	"strings"
	"time"

	"bentos-backend/shared/logger/stdlogger"
	"bentos-backend/usecase"
)

// AutogenCommentClient posts autogen comments to GitHub PRs.
type AutogenCommentClient interface {
	CreateComment(ctx context.Context, repository string, pullRequestNumber int, body string) error
}

// AutogenPublisher publishes autogen output to GitHub.
type AutogenPublisher struct {
	client AutogenCommentClient
	logger usecase.Logger
}

// NewAutogenPublisher creates a GitHub autogen publisher.
func NewAutogenPublisher(client AutogenCommentClient, logger usecase.Logger) *AutogenPublisher {
	if logger == nil {
		logger = stdlogger.Nop()
	}
	return &AutogenPublisher{client: client, logger: logger}
}

// PublishAutogen posts a summary comment and pushes changes to the PR head branch.
func (p *AutogenPublisher) PublishAutogen(ctx context.Context, req usecase.AutogenPublishRequest) error {
	if !req.Publish {
		return nil
	}
	if p.client == nil {
		return fmt.Errorf("autogen comment client is not configured")
	}
	if req.Target.ChangeRequestNumber <= 0 {
		return fmt.Errorf("pull request number must be positive")
	}
	headBranch := strings.TrimSpace(req.HeadBranch)
	if headBranch == "" {
		return fmt.Errorf("head branch is required for autogen publish")
	}
	if req.Environment == nil {
		return fmt.Errorf("code environment is required for autogen publish")
	}

	startedAt := time.Now()
	p.logger.Infof("Publishing GitHub autogen result started.")
	p.logger.Debugf("Repository is %q and change request number is %d.", req.Target.Repository, req.Target.ChangeRequestNumber)

	body := buildAutogenSummaryBody(req)
	if err := p.client.CreateComment(ctx, req.Target.Repository, req.Target.ChangeRequestNumber, body); err != nil {
		p.logger.Errorf("Publishing GitHub autogen summary failed.")
		p.logger.Debugf("Failure details: %v.", err)
		return err
	}

	if _, err := req.Environment.PushChanges(ctx, req.PushOptions); err != nil {
		return err
	}

	p.logger.Infof("Publishing GitHub autogen result completed.")
	p.logger.Debugf("The publish operation completed in %d ms.", time.Since(startedAt).Milliseconds())
	return nil
}

func buildAutogenSummaryBody(req usecase.AutogenPublishRequest) string {
	summary := fmt.Sprintf(
		"Autogen summary\n\nTests added:\n%s\n\nDocs added:\n%s\n\nComments added:\n%s",
		formatSummaryList(req.Summary.Tests),
		formatSummaryList(req.Summary.Docs),
		formatSummaryList(req.Summary.Comments),
	)
	agentOutput := strings.TrimSpace(req.AgentOutput)
	if agentOutput == "" {
		return summary
	}
	return fmt.Sprintf("%s\n\nAgent output\n\n%s", summary, agentOutput)
}

func formatSummaryList(items []string) string {
	if len(items) == 0 {
		return "- none"
	}
	var builder strings.Builder
	for _, item := range items {
		builder.WriteString("- ")
		builder.WriteString(item)
		builder.WriteString("\n")
	}
	return strings.TrimRight(builder.String(), "\n")
}
