package github

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"

	githubvcs "bentos-backend/adapter/outbound/vcs/github"
	"bentos-backend/domain"
	"bentos-backend/shared/text"
)

func resolveWebhookIssueCandidates(
	ctx context.Context,
	client CommentClient,
	repository string,
	description string,
) []domain.IssueContext {
	references := text.ExtractIssueReferences(description, repository)
	if len(references) == 0 {
		return nil
	}

	candidates := make([]domain.IssueContext, 0, len(references))
	for _, ref := range references {
		issue, err := client.GetIssue(ctx, ref.Repository, ref.Number)
		if err != nil {
			continue
		}
		comments, err := client.ListIssueComments(ctx, ref.Repository, ref.Number)
		if err != nil {
			continue
		}
		issueComments := make([]domain.Comment, 0, len(comments))
		for _, comment := range comments {
			issueComments = append(issueComments, comment.ToDomain())
		}
		candidates = append(candidates, domain.IssueContext{
			Issue:    issue.ToDomain(),
			Comments: issueComments,
		})
	}
	if len(candidates) == 0 {
		return nil
	}
	return candidates
}

func isValidPullRequestEvent(event pullRequestEvent) bool {
	return strings.TrimSpace(event.Repository.FullName) != "" &&
		strings.TrimSpace(event.Repository.CloneURL) != "" &&
		event.PullRequest.Number > 0 &&
		strings.TrimSpace(event.PullRequest.Base.Ref) != "" &&
		strings.TrimSpace(event.PullRequest.Head.Ref) != ""
}

func isValidIssueCommentEvent(event issueCommentEvent) bool {
	return strings.TrimSpace(event.Repository.FullName) != "" &&
		strings.TrimSpace(event.Repository.CloneURL) != "" &&
		event.Issue.Number > 0 &&
		event.Issue.PullRequest != nil &&
		event.Comment.ID > 0
}

func isValidReviewCommentEvent(event reviewCommentEvent) bool {
	return strings.TrimSpace(event.Repository.FullName) != "" &&
		strings.TrimSpace(event.Repository.CloneURL) != "" &&
		event.PullRequest.Number > 0 &&
		event.Comment.ID > 0
}

func isBotAuthor(authorType string) bool {
	return strings.EqualFold(strings.TrimSpace(authorType), "bot")
}

func buildIssueThreadForWebhook(ctx context.Context, client CommentClient, repository string, prNumber int, commentID int64, prInfo githubvcs.PullRequestInfo) (domain.CommentThread, error) {
	comments, err := client.ListIssueComments(ctx, repository, prNumber)
	if err != nil {
		return domain.CommentThread{}, err
	}
	threadComments := make([]domain.Comment, 0, len(comments))
	for _, comment := range comments {
		threadComments = append(threadComments, comment.ToDomain())
	}
	sort.Slice(threadComments, func(i, j int) bool {
		return threadComments[i].CreatedAt.Before(threadComments[j].CreatedAt)
	})
	return domain.CommentThread{
		Kind:     domain.CommentKindIssue,
		RootID:   commentID,
		Context:  buildIssueThreadContext(prInfo),
		Comments: threadComments,
	}, nil
}

func buildReviewThreadForWebhook(ctx context.Context, client CommentClient, repository string, prNumber int, commentID int64) (domain.CommentThread, error) {
	comments, err := client.ListReviewComments(ctx, repository, prNumber)
	if err != nil {
		return domain.CommentThread{}, err
	}
	byID := make(map[int64]githubvcs.ReviewComment, len(comments))
	for _, comment := range comments {
		byID[comment.ID] = comment
	}
	rootID := resolveReviewRootID(byID, commentID)
	threadComments := make([]domain.Comment, 0, len(comments))
	var root githubvcs.ReviewComment
	if comment, ok := byID[rootID]; ok {
		root = comment
	}
	reviewSummary := githubvcs.PullRequestReviewSummary{}
	if root.ReviewID > 0 {
		if summary, err := client.GetPullRequestReview(ctx, repository, prNumber, root.ReviewID); err == nil {
			reviewSummary = summary
		}
	}
	for _, comment := range comments {
		if resolveReviewRootID(byID, comment.ID) == rootID {
			threadComments = append(threadComments, comment.ToDomain())
		}
	}
	sort.Slice(threadComments, func(i, j int) bool {
		return threadComments[i].CreatedAt.Before(threadComments[j].CreatedAt)
	})
	return domain.CommentThread{
		Kind:     domain.CommentKindReview,
		RootID:   rootID,
		Context:  buildReviewThreadContext(root, reviewSummary),
		Comments: threadComments,
	}, nil
}

func resolveReviewRootID(byID map[int64]githubvcs.ReviewComment, commentID int64) int64 {
	currentID := commentID
	for {
		comment, ok := byID[currentID]
		if !ok || comment.InReplyToID == 0 {
			return currentID
		}
		currentID = comment.InReplyToID
	}
}

func buildIssueThreadContext(prInfo githubvcs.PullRequestInfo) []string {
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

func buildReviewThreadContext(root githubvcs.ReviewComment, reviewSummary githubvcs.PullRequestReviewSummary) []string {
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

func formatReviewLineInfo(root githubvcs.ReviewComment) string {
	if root.Line > 0 {
		return fmt.Sprintf("Line: %d (%s)", root.Line, strings.TrimSpace(root.Side))
	}
	if root.OriginalLine > 0 {
		return fmt.Sprintf("Original Line: %d", root.OriginalLine)
	}
	return ""
}

func formatReviewSummary(summary githubvcs.PullRequestReviewSummary) string {
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

func isActionAllowed(value string, allowlist []string, defaultAllowlist []string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return false
	}
	if allowlist == nil {
		return containsNormalized(defaultAllowlist, normalized)
	}
	return containsNormalized(allowlist, normalized)
}

func containsNormalized(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), target) {
			return true
		}
	}
	return false
}

func buildAuthenticatedCloneURL(rawCloneURL string, installationToken string) (string, error) {
	installationToken = strings.TrimSpace(installationToken)
	if installationToken == "" {
		return "", fmt.Errorf("installation token is required")
	}
	cloneURL, err := url.Parse(strings.TrimSpace(rawCloneURL))
	if err != nil {
		return "", err
	}
	if cloneURL.Scheme != "http" && cloneURL.Scheme != "https" {
		return "", fmt.Errorf("clone URL must use http or https")
	}
	cloneURL.User = url.UserPassword("x-access-token", installationToken)
	return cloneURL.String(), nil
}
