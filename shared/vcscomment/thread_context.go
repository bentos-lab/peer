package vcscomment

import (
	"fmt"
	"strings"

	"github.com/bentos-lab/peer/domain"
)

// BuildIssueThreadContext builds a thread context from a change request description.
func BuildIssueThreadContext(prInfo domain.ChangeRequestInfo) []string {
	title := strings.TrimSpace(prInfo.Title)
	description := strings.TrimSpace(prInfo.Description)
	if title == "" && description == "" {
		return nil
	}
	lines := []string{"PR Description:"}
	if title != "" {
		lines = append(lines, fmt.Sprintf("Title: %s", title))
	}
	if description != "" {
		lines = append(lines, description)
	}
	return lines
}

// BuildReviewThreadContext builds a thread context for code review comments.
func BuildReviewThreadContext(root domain.ReviewComment, reviewSummary domain.ReviewSummary) []string {
	lines := make([]string, 0)
	if strings.TrimSpace(root.Path) != "" {
		lines = append(lines, fmt.Sprintf("File: %s", strings.TrimSpace(root.Path)))
	}
	lineInfo := formatReviewLineInfo(root)
	if lineInfo != "" {
		lines = append(lines, lineInfo)
	}
	if strings.TrimSpace(root.DiffHunk) != "" {
		lines = append(lines, "Diff Hunk:", "```diff", root.DiffHunk, "```")
	}
	if summary := formatReviewSummary(reviewSummary); summary != "" {
		lines = append(lines, "Review Summary:", summary)
	}
	if len(lines) == 0 {
		return nil
	}
	return lines
}

// ResolveReviewRootID resolves the top-level review comment for a thread.
func ResolveReviewRootID(byID map[int64]domain.ReviewComment, commentID int64) int64 {
	currentID := commentID
	for {
		comment, ok := byID[currentID]
		if !ok || comment.InReplyToID == 0 {
			return currentID
		}
		currentID = comment.InReplyToID
	}
}

func formatReviewLineInfo(root domain.ReviewComment) string {
	if root.Line > 0 {
		return fmt.Sprintf("Line: %d (%s)", root.Line, strings.TrimSpace(root.Side))
	}
	if root.OriginalLine > 0 {
		return fmt.Sprintf("Original Line: %d", root.OriginalLine)
	}
	return ""
}

func formatReviewSummary(summary domain.ReviewSummary) string {
	body := strings.TrimSpace(summary.Body)
	if body == "" {
		return ""
	}
	state := strings.TrimSpace(summary.State)
	author := strings.TrimSpace(summary.User.Login)
	if state != "" || author != "" {
		prefix := "Review"
		if state != "" {
			prefix = fmt.Sprintf("%s (%s)", prefix, state)
		}
		if author != "" {
			prefix = fmt.Sprintf("%s by %s", prefix, author)
		}
		return fmt.Sprintf("%s:\n%s", prefix, body)
	}
	return body
}
