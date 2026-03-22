package codeagent

import (
	"strings"

	"github.com/bentos-lab/peer/domain"
)

func normalizeKeyIdeas(ideas []string) []string {
	if len(ideas) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(ideas))
	filtered := make([]string, 0, len(ideas))
	for _, idea := range ideas {
		trimmed := strings.TrimSpace(idea)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		filtered = append(filtered, trimmed)
	}
	return filtered
}

func fallbackIssueReference(candidates []domain.IssueContext) domain.IssueReference {
	if len(candidates) == 0 {
		return domain.IssueReference{}
	}
	issue := candidates[0].Issue
	return domain.IssueReference{
		Repository: issue.Repository,
		Number:     issue.Number,
		Title:      issue.Title,
	}
}
