package github

import (
	"strings"
)

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
