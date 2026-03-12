package cli

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

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
	replyCommentUseCaseBuilder ReplyCommentUseCaseBuilder
	githubClient               ReplyCommentGitHubClient
	triggerName                string
}

// ReplyCommentUseCaseBuilder builds a reply comment usecase for a specific repo.
type ReplyCommentUseCaseBuilder func(repoURL string) (usecase.ReplyCommentUseCase, error)

// ReplyCommentRunParams contains already-parsed replycomment parameters.
type ReplyCommentRunParams struct {
	VCSProvider   string
	Repo          string
	ChangeRequest string
	CommentID     string
	Question      string
	Comment       bool
}

// NewReplyCommentCommand creates a new CLI command for replycomment.
func NewReplyCommentCommand(replyCommentUseCaseBuilder ReplyCommentUseCaseBuilder, githubClient ReplyCommentGitHubClient, triggerName string) *ReplyCommentCommand {
	return &ReplyCommentCommand{
		replyCommentUseCaseBuilder: replyCommentUseCaseBuilder,
		githubClient:               githubClient,
		triggerName:                strings.TrimSpace(triggerName),
	}
}

// Run executes the CLI replycomment flow.
func (c *ReplyCommentCommand) Run(ctx context.Context, params ReplyCommentRunParams) error {
	if c.replyCommentUseCaseBuilder == nil {
		return errors.New("replycomment usecase is not configured")
	}
	if c.githubClient == nil {
		return errors.New("github client is not configured")
	}

	provider := strings.TrimSpace(strings.ToLower(params.VCSProvider))
	if provider == "" {
		provider = "github"
	}
	if provider != "github" {
		return fmt.Errorf("unsupported vcs provider: %s", provider)
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

	replyCommentUseCase, err := c.replyCommentUseCaseBuilder(repoURL)
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
		_, err = replyCommentUseCase.Execute(ctx, request)
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
		_, err = replyCommentUseCase.Execute(ctx, request)
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
	_, err = replyCommentUseCase.Execute(ctx, request)
	return err
}
