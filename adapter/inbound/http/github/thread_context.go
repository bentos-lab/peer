package github

import (
	"context"
	"sort"

	"github.com/bentos-lab/peer/domain"
	"github.com/bentos-lab/peer/shared/vcscomment"
)

func buildIssueThreadForWebhook(ctx context.Context, client CommentClient, repository string, prNumber int, commentID int64, prInfo domain.ChangeRequestInfo) (domain.CommentThread, error) {
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
		Context:  vcscomment.BuildIssueThreadContext(prInfo),
		Comments: threadComments,
	}, nil
}

func buildReviewThreadForWebhook(ctx context.Context, client CommentClient, repository string, prNumber int, commentID int64) (domain.CommentThread, error) {
	comments, err := client.ListReviewComments(ctx, repository, prNumber)
	if err != nil {
		return domain.CommentThread{}, err
	}
	byID := make(map[int64]domain.ReviewComment, len(comments))
	for _, comment := range comments {
		byID[comment.ID] = comment
	}
	rootID := vcscomment.ResolveReviewRootID(byID, commentID)
	threadComments := make([]domain.Comment, 0, len(comments))
	var root domain.ReviewComment
	if comment, ok := byID[rootID]; ok {
		root = comment
	}
	reviewSummary := domain.ReviewSummary{}
	if root.ReviewID > 0 {
		if summary, err := client.GetPullRequestReview(ctx, repository, prNumber, root.ReviewID); err == nil {
			reviewSummary = summary
		}
	}
	for _, comment := range comments {
		if vcscomment.ResolveReviewRootID(byID, comment.ID) == rootID {
			threadComments = append(threadComments, comment.ToDomain())
		}
	}
	sort.Slice(threadComments, func(i, j int) bool {
		return threadComments[i].CreatedAt.Before(threadComments[j].CreatedAt)
	})
	return domain.CommentThread{
		Kind:     domain.CommentKindReview,
		RootID:   rootID,
		Context:  vcscomment.BuildReviewThreadContext(root, reviewSummary),
		Comments: threadComments,
	}, nil
}
