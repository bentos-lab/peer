package codeagent

import (
	"strings"

	"github.com/bentos-lab/peer/domain"
	sharedtext "github.com/bentos-lab/peer/shared/text"
)

func mapIssueCandidates(candidates []domain.IssueContext) []issueAlignmentIssueData {
	if len(candidates) == 0 {
		return nil
	}
	mapped := make([]issueAlignmentIssueData, 0, len(candidates))
	for _, candidate := range candidates {
		issue := candidate.Issue
		mapped = append(mapped, issueAlignmentIssueData{
			Repository: issue.Repository,
			Number:     issue.Number,
			Title:      issue.Title,
			Body:       sharedtext.SingleLine(issue.Body),
			Comments:   mapIssueComments(candidate.Comments),
		})
	}
	return mapped
}

func mapIssueComments(comments []domain.Comment) []domain.Comment {
	if len(comments) == 0 {
		return nil
	}
	mapped := make([]domain.Comment, 0, len(comments))
	for _, comment := range comments {
		if strings.TrimSpace(comment.Body) == "" {
			continue
		}
		mapped = append(mapped, domain.Comment{
			Author: comment.Author,
			Body:   sharedtext.SingleLine(comment.Body),
		})
	}
	return mapped
}
