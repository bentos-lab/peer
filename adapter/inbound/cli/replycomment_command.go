package cli

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	githubvcs "bentos-backend/adapter/outbound/vcs/github"
	"bentos-backend/domain"
	"bentos-backend/shared/text"
	"bentos-backend/usecase"
)

// ReplyCommentGitHubClient resolves repository and pull-request metadata for replycomment.
type ReplyCommentGitHubClient interface {
	ResolveRepository(ctx context.Context, repository string) (string, error)
	GetPullRequestInfo(ctx context.Context, repository string, pullRequestNumber int) (githubvcs.PullRequestInfo, error)
	GetPullRequestReview(ctx context.Context, repository string, pullRequestNumber int, reviewID int64) (githubvcs.PullRequestReviewSummary, error)
	GetIssueComment(ctx context.Context, repository string, commentID int64) (githubvcs.IssueComment, error)
	GetReviewComment(ctx context.Context, repository string, commentID int64) (githubvcs.ReviewComment, error)
	ListIssueComments(ctx context.Context, repository string, pullRequestNumber int) ([]githubvcs.IssueComment, error)
	ListReviewComments(ctx context.Context, repository string, pullRequestNumber int) ([]githubvcs.ReviewComment, error)
}

// ReplyCommentCommand runs the replycomment flow.
type ReplyCommentCommand struct {
	replyCommentUseCase usecase.ReplyCommentUseCase
	githubClient        ReplyCommentGitHubClient
	triggerName         string
}

// ReplyCommentRunParams contains already-parsed replycomment parameters.
type ReplyCommentRunParams struct {
	Provider      string
	Repo          string
	ChangeRequest string
	CommentID     string
	Question      string
	Comment       bool
}

// NewReplyCommentCommand creates a new CLI command for replycomment.
func NewReplyCommentCommand(replyCommentUseCase usecase.ReplyCommentUseCase, githubClient ReplyCommentGitHubClient, triggerName string) *ReplyCommentCommand {
	return &ReplyCommentCommand{
		replyCommentUseCase: replyCommentUseCase,
		githubClient:        githubClient,
		triggerName:         strings.TrimSpace(triggerName),
	}
}

// Run executes the CLI replycomment flow.
func (c *ReplyCommentCommand) Run(ctx context.Context, params ReplyCommentRunParams) error {
	if c.replyCommentUseCase == nil {
		return errors.New("replycomment usecase is not configured")
	}
	if c.githubClient == nil {
		return errors.New("github client is not configured")
	}

	provider := strings.TrimSpace(strings.ToLower(params.Provider))
	if provider == "" {
		provider = "github"
	}
	if provider != "github" {
		return fmt.Errorf("unsupported provider: %s", provider)
	}

	if strings.TrimSpace(params.ChangeRequest) == "" {
		return errors.New("--change-request is required")
	}
	if strings.TrimSpace(params.CommentID) != "" && strings.TrimSpace(params.Question) != "" {
		return errors.New("--comment-id and --question are mutually exclusive")
	}
	if strings.TrimSpace(params.Question) != "" && params.Comment {
		return errors.New("--comment is not supported with --question")
	}
	if strings.TrimSpace(params.CommentID) == "" && strings.TrimSpace(params.Question) == "" {
		return errors.New("either --comment-id or --question is required")
	}

	prNumber, err := strconv.Atoi(strings.TrimSpace(params.ChangeRequest))
	if err != nil || prNumber <= 0 {
		return fmt.Errorf("--change-request must be a positive integer")
	}

	repository, repoURL, _, err := normalizeRepo(params.Repo)
	if err != nil {
		return err
	}
	repository, err = c.githubClient.ResolveRepository(ctx, repository)
	if err != nil {
		return err
	}

	prInfo, err := c.githubClient.GetPullRequestInfo(ctx, repository, prNumber)
	if err != nil {
		return err
	}

	request := usecase.ReplyCommentRequest{
		Repository:          prInfo.Repository,
		RepoURL:             repoURL,
		ChangeRequestNumber: prNumber,
		Title:               prInfo.Title,
		Description:         prInfo.Description,
		Base:                prInfo.BaseRef,
		Head:                prInfo.HeadRef,
		Publish:             params.Comment,
	}

	if strings.TrimSpace(params.Question) != "" {
		request.Question = strings.TrimSpace(params.Question)
		request.CommentKind = domain.CommentKindIssue
		request.Thread = buildInlineQuestionThread(request.Question, prInfo)
		_, err = c.replyCommentUseCase.Execute(ctx, request)
		return err
	}

	commentID, err := parseCommentID(params.CommentID)
	if err != nil {
		return err
	}
	request.CommentID = commentID

	reviewComment, reviewErr := c.githubClient.GetReviewComment(ctx, prInfo.Repository, commentID)
	if reviewErr == nil && reviewComment.ID > 0 {
		request.CommentKind = domain.CommentKindReview
		request.Question = text.StripTrigger(reviewComment.Body, c.triggerName)
		thread, err := buildReviewThread(ctx, c.githubClient, prInfo.Repository, prNumber, commentID)
		if err != nil {
			return err
		}
		request.Thread = thread
		_, err = c.replyCommentUseCase.Execute(ctx, request)
		return err
	}

	issueComment, issueErr := c.githubClient.GetIssueComment(ctx, prInfo.Repository, commentID)
	if issueErr != nil || issueComment.ID <= 0 {
		if reviewErr != nil {
			return fmt.Errorf("failed to resolve comment: %v", reviewErr)
		}
		return fmt.Errorf("failed to resolve comment: %v", issueErr)
	}
	request.CommentKind = domain.CommentKindIssue
	request.Question = text.StripTrigger(issueComment.Body, c.triggerName)
	thread, err := buildIssueThread(ctx, c.githubClient, prInfo.Repository, prNumber, commentID, prInfo)
	if err != nil {
		return err
	}
	request.Thread = thread
	_, err = c.replyCommentUseCase.Execute(ctx, request)
	return err
}

func buildInlineQuestionThread(question string, prInfo githubvcs.PullRequestInfo) domain.CommentThread {
	now := time.Now()
	return domain.CommentThread{
		Kind:    domain.CommentKindIssue,
		RootID:  0,
		Context: buildIssueThreadContext(prInfo),
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
	}

	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("--comment-id must be a positive integer or discussion anchor")
	}
	return parsed, nil
}

func buildIssueThread(ctx context.Context, client ReplyCommentGitHubClient, repository string, prNumber int, commentID int64, prInfo githubvcs.PullRequestInfo) (domain.CommentThread, error) {
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

func buildReviewThread(ctx context.Context, client ReplyCommentGitHubClient, repository string, prNumber int, commentID int64) (domain.CommentThread, error) {
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
