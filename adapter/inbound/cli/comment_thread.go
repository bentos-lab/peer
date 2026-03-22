package cli

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bentos-lab/peer/domain"
	"github.com/bentos-lab/peer/shared/vcscomment"
)

func buildInlineQuestionThread(question string, prInfo domain.ChangeRequestInfo) domain.CommentThread {
	now := time.Now()
	return domain.CommentThread{
		Kind:    domain.CommentKindIssue,
		RootID:  0,
		Context: vcscomment.BuildIssueThreadContext(prInfo),
		Comments: []domain.Comment{{
			ID:        0,
			Body:      strings.TrimSpace(question),
			Author:    domain.CommentAuthor{Login: "cli"},
			CreatedAt: now,
		}},
	}
}

func parseCommentID(raw string) (int64, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, fmt.Errorf("--comment-id must be a positive integer")
	}

	if strings.Contains(value, "#") {
		parts := strings.SplitN(value, "#", 2)
		if len(parts) == 2 {
			value = parts[1]
		}
	}

	switch {
	case strings.HasPrefix(value, "discussion_r"):
		value = strings.TrimPrefix(value, "discussion_r")
	case strings.HasPrefix(value, "issuecomment-"):
		value = strings.TrimPrefix(value, "issuecomment-")
	case strings.HasPrefix(value, "note_"):
		value = strings.TrimPrefix(value, "note_")
	case strings.HasPrefix(value, "note-"):
		value = strings.TrimPrefix(value, "note-")
	}

	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("--comment-id must be a positive integer or discussion anchor")
	}
	return parsed, nil
}

func buildIssueThread(ctx context.Context, client VCSClient, repository string, prNumber int, commentID int64, prInfo domain.ChangeRequestInfo) (domain.CommentThread, error) {
	comments, err := client.ListChangeRequestComments(ctx, repository, prNumber)
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

func buildReviewThread(ctx context.Context, client VCSClient, repository string, prNumber int, commentID int64) (domain.CommentThread, error) {
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
