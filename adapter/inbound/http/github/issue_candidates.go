package github

import (
	"context"

	"github.com/bentos-lab/peer/domain"
	"github.com/bentos-lab/peer/shared/text"
)

func resolveWebhookIssueCandidates(
	ctx context.Context,
	client CommentClient,
	repository string,
	description string,
) []domain.IssueContext {
	references := text.ExtractGitHubIssueReferences(description, repository)
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
			Issue:    issue,
			Comments: issueComments,
		})
	}
	if len(candidates) == 0 {
		return nil
	}
	return candidates
}
