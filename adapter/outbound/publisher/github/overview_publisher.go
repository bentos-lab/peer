package github

import (
	"context"
	"fmt"
	"strings"

	"github.com/bentos-lab/peer/domain"
	"github.com/bentos-lab/peer/shared/logger/stdlogger"
	"github.com/bentos-lab/peer/usecase"
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
	if !shouldPublishOverviewForAction(req.Metadata["action"]) {
		p.logger.Infof(
			"Skipped GitHub overview comment for %q#%d action=%q.",
			req.Target.Repository,
			req.Target.ChangeRequestNumber,
			req.Metadata["action"],
		)
		return nil
	}

	body := formatOverviewMarkdown(req.Overview, req.IssueAlignment)
	if warning := usecase.FormatRecipeWarning(req.RecipeWarnings); warning != "" {
		body = fmt.Sprintf("%s\n\n%s", warning, body)
	}
	if err := p.client.CreateComment(ctx, req.Target.Repository, req.Target.ChangeRequestNumber, body); err != nil {
		p.logOverviewPayload("failed", req, body)
		return err
	}
	p.logOverviewPayload("success", req, body)
	return nil
}

func (p *OverviewPublisher) logOverviewPayload(state string, req usecase.OverviewPublishRequest, body string) {
	p.logger.Debugf("GitHub overview comment metadata state=%q repo=%q pr=%d action=%q categories=%d walkthroughs=%d.",
		state, req.Target.Repository, req.Target.ChangeRequestNumber, req.Metadata["action"], len(req.Overview.Categories), len(req.Overview.Walkthroughs))
	p.logger.Tracef("GitHub overview comment content state=%q body=%q.", state, body)
}

func shouldPublishOverviewForAction(action string) bool {
	normalized := strings.TrimSpace(strings.ToLower(action))
	if normalized == "" {
		return true
	}
	return normalized == "opened"
}

func formatOverviewMarkdown(result usecase.LLMOverviewResult, alignment *domain.IssueAlignmentResult) string {
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

	if issueSection := formatIssueAlignmentMarkdown(alignment); issueSection != "" {
		builder.WriteString("\n")
		builder.WriteString(issueSection)
	}

	return builder.String()
}

func formatIssueAlignmentMarkdown(alignment *domain.IssueAlignmentResult) string {
	if alignment == nil || len(alignment.Requirements) == 0 {
		return ""
	}

	var builder strings.Builder
	builder.WriteString("## Issue Alignment\n\n")
	if strings.TrimSpace(alignment.Issue.Title) == "" {
		builder.WriteString(fmt.Sprintf("Linked issue: #%d\n\n", alignment.Issue.Number))
	} else {
		builder.WriteString(fmt.Sprintf("Linked issue: #%d - %s\n\n", alignment.Issue.Number, escapeMarkdownText(alignment.Issue.Title)))
	}
	if strings.TrimSpace(alignment.Issue.Repository) != "" {
		builder.WriteString(fmt.Sprintf("Repository: %s\n\n", escapeMarkdownText(alignment.Issue.Repository)))
	}
	builder.WriteString("| Requirement | Coverage |\n")
	builder.WriteString("| --- | --- |\n")
	for _, row := range alignment.Requirements {
		coverage := formatIssueAlignmentCoverage(row.Coverage)
		builder.WriteString(fmt.Sprintf("| %s | %s |\n", escapeTableCell(row.Requirement), escapeTableCell(coverage)))
	}
	return builder.String()
}

func formatIssueAlignmentCoverage(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return trimmed
	}
	normalized := strings.ToUpper(trimmed)
	switch {
	case strings.HasPrefix(normalized, "NOT COVERED"):
		return fmt.Sprintf(":x: %s", trimmed)
	case strings.HasPrefix(normalized, "PARTIAL"):
		return fmt.Sprintf(":warning: %s", trimmed)
	case strings.HasPrefix(normalized, "COVERED"):
		return fmt.Sprintf(":white_check_mark: %s", trimmed)
	default:
		return trimmed
	}
}

func escapeMarkdownText(value string) string {
	return strings.TrimSpace(strings.ReplaceAll(value, "\n", " "))
}

func escapeTableCell(value string) string {
	replaced := strings.ReplaceAll(value, "|", "\\|")
	replaced = strings.ReplaceAll(replaced, "\n", "<br>")
	return strings.TrimSpace(replaced)
}
