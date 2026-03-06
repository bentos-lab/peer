package github

import (
	"context"
	"fmt"
	"strings"
	"time"

	"bentos-backend/shared/logger/stdlogger"
	"bentos-backend/usecase"
)

// OverviewCommentClient posts overview comments to GitHub PRs.
type OverviewCommentClient interface {
	CreateComment(ctx context.Context, repository string, pullRequestNumber int, body string) error
}

// OverviewPublisher publishes overview comments to GitHub pull requests.
type OverviewPublisher struct {
	client OverviewCommentClient
	logger usecase.Logger
}

// NewOverviewPublisher creates a GitHub overview publisher.
func NewOverviewPublisher(client OverviewCommentClient, logger usecase.Logger) *OverviewPublisher {
	if logger == nil {
		logger = stdlogger.Nop()
	}
	return &OverviewPublisher{client: client, logger: logger}
}

// PublishOverview posts a markdown overview comment to one GitHub pull request.
func (p *OverviewPublisher) PublishOverview(ctx context.Context, req usecase.OverviewPublishRequest) error {
	startedAt := time.Now()
	if !shouldPublishOverviewForAction(req.Metadata["action"]) {
		p.logger.Infof("Skipped GitHub overview comment because webhook action is not initial creation.")
		p.logger.Debugf("Repository is %q and change request number is %d.", req.Target.Repository, req.Target.ChangeRequestNumber)
		return nil
	}

	body := formatOverviewMarkdown(req.Overview)
	if err := p.client.CreateComment(ctx, req.Target.Repository, req.Target.ChangeRequestNumber, body); err != nil {
		p.logger.Errorf("Publishing GitHub overview failed.")
		p.logger.Debugf("Repository is %q and change request number is %d.", req.Target.Repository, req.Target.ChangeRequestNumber)
		p.logger.Debugf("The publish operation ran for %d ms before failing.", time.Since(startedAt).Milliseconds())
		p.logger.Debugf("Failure details: %v.", err)
		return err
	}

	p.logger.Infof("Publishing GitHub overview completed.")
	p.logger.Debugf("Repository is %q and change request number is %d.", req.Target.Repository, req.Target.ChangeRequestNumber)
	p.logger.Debugf("The publish operation completed in %d ms.", time.Since(startedAt).Milliseconds())
	return nil
}

func shouldPublishOverviewForAction(action string) bool {
	normalized := strings.TrimSpace(strings.ToLower(action))
	if normalized == "" {
		return true
	}
	return normalized == "opened"
}

func formatOverviewMarkdown(result usecase.LLMOverviewResult) string {
	var builder strings.Builder
	builder.WriteString("## Summary\n\n")
	if len(result.Categories) == 0 {
		builder.WriteString("- No significant high-level changes identified.\n")
	} else {
		for _, item := range result.Categories {
			builder.WriteString(fmt.Sprintf("- **%s**: %s\n", escapeMarkdownText(string(item.Category)), escapeMarkdownText(item.Summary)))
		}
	}

	builder.WriteString("\n## Walkthroughs\n\n")
	builder.WriteString("| Group | Summary |\n")
	builder.WriteString("| --- | --- |\n")
	if len(result.Walkthroughs) == 0 {
		builder.WriteString("| No grouped walkthroughs generated | No additional walkthrough details were generated. |\n")
		return builder.String()
	}

	for _, group := range result.Walkthroughs {
		left := fmt.Sprintf("**%s**", escapeTableCell(group.GroupName))
		if len(group.Files) > 0 {
			quotedFiles := make([]string, 0, len(group.Files))
			for _, file := range group.Files {
				quotedFiles = append(quotedFiles, fmt.Sprintf("`%s`", escapeTableCell(file)))
			}
			left = fmt.Sprintf("%s<br>%s", left, strings.Join(quotedFiles, "<br>"))
		}
		builder.WriteString(fmt.Sprintf("| %s | %s |\n", left, escapeTableCell(group.Summary)))
	}

	return builder.String()
}

func escapeMarkdownText(value string) string {
	return strings.TrimSpace(strings.ReplaceAll(value, "\n", " "))
}

func escapeTableCell(value string) string {
	replaced := strings.ReplaceAll(value, "|", "\\|")
	replaced = strings.ReplaceAll(replaced, "\n", "<br>")
	return strings.TrimSpace(replaced)
}
