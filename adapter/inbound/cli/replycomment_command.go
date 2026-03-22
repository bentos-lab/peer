package cli

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/bentos-lab/peer/config"
	"github.com/bentos-lab/peer/domain"
	"github.com/bentos-lab/peer/shared/logger/stdlogger"
	"github.com/bentos-lab/peer/shared/text"
	"github.com/bentos-lab/peer/usecase"
	uccontracts "github.com/bentos-lab/peer/usecase/contracts"
)

// ReplyCommentCommand runs the replycomment flow.
type ReplyCommentCommand struct {
	replyCommentUseCaseBuilder ReplyCommentUseCaseBuilder
	vcsResolver                VCSClientResolver
	envFactory                 uccontracts.CodeEnvironmentFactory
	recipeLoader               usecase.CustomRecipeLoader
	triggerName                string
	logger                     usecase.Logger
}

// ReplyCommentUseCaseBuilder builds a reply comment usecase for a specific repo.
type ReplyCommentUseCaseBuilder func(repoURL string) (usecase.ReplyCommentUseCase, error)

// ReplyCommentRunParams contains already-parsed replycomment parameters.
type ReplyCommentRunParams struct {
	VCSProvider   string
	VCSHost       string
	Repo          string
	ChangeRequest string
	CommentID     string
	Question      string
	Publish       bool
}

// NewReplyCommentCommand creates a new CLI command for replycomment.
func NewReplyCommentCommand(replyCommentUseCaseBuilder ReplyCommentUseCaseBuilder, vcsResolver VCSClientResolver, envFactory uccontracts.CodeEnvironmentFactory, recipeLoader usecase.CustomRecipeLoader, triggerName string, logger usecase.Logger) *ReplyCommentCommand {
	if logger == nil {
		logger = stdlogger.Nop()
	}
	return &ReplyCommentCommand{
		replyCommentUseCaseBuilder: replyCommentUseCaseBuilder,
		vcsResolver:                vcsResolver,
		envFactory:                 envFactory,
		recipeLoader:               recipeLoader,
		triggerName:                strings.TrimSpace(triggerName),
		logger:                     logger,
	}
}

// Run executes the CLI replycomment flow.
func (c *ReplyCommentCommand) Run(ctx context.Context, cfg config.Config, params ReplyCommentRunParams) error {
	if c.replyCommentUseCaseBuilder == nil {
		return errors.New("replycomment usecase is not configured")
	}
	if c.vcsResolver == nil {
		return errors.New("vcs client resolver is not configured")
	}
	if c.envFactory == nil {
		return errors.New("code environment factory is not configured")
	}
	if c.recipeLoader == nil {
		return errors.New("recipe loader is not configured")
	}
	if c.logger == nil {
		c.logger = stdlogger.Nop()
	}
	_ = cfg

	vcsClient, err := c.vcsResolver.Resolve(params.VCSProvider)
	if err != nil {
		return err
	}

	if strings.TrimSpace(params.ChangeRequest) == "" {
		return errors.New("--change-request is required")
	}
	if strings.TrimSpace(params.CommentID) != "" && strings.TrimSpace(params.Question) != "" {
		return errors.New("--comment-id and --question are mutually exclusive")
	}
	if strings.TrimSpace(params.Question) != "" && params.Publish {
		return errors.New("--publish is not supported with --question")
	}
	if strings.TrimSpace(params.CommentID) == "" && strings.TrimSpace(params.Question) == "" {
		return errors.New("either --comment-id or --question is required")
	}

	prNumber, err := strconv.Atoi(strings.TrimSpace(params.ChangeRequest))
	if err != nil || prNumber <= 0 {
		return fmt.Errorf("--change-request must be a positive integer")
	}

	repository, repoURL, _, err := normalizeRepo(normalizeVCSProvider(params.VCSProvider), params.VCSHost, params.Repo)
	if err != nil {
		return err
	}

	if c.envFactory == nil {
		return fmt.Errorf("code environment factory is required")
	}
	environment, err := c.envFactory.New(ctx, domain.CodeEnvironmentInitOptions{
		RepoURL: repoURL,
	})
	if err != nil {
		return err
	}
	cleanup := environment.Cleanup
	defer func() {
		if cleanupErr := cleanup(ctx); cleanupErr != nil {
			c.logger.Warnf("Failed to cleanup code environment: %v", cleanupErr)
		}
	}()

	replyCommentUseCase, err := c.replyCommentUseCaseBuilder(repoURL)
	if err != nil {
		return err
	}
	repository, err = vcsClient.ResolveRepository(ctx, repository)
	if err != nil {
		return err
	}

	prInfo, err := vcsClient.GetPullRequestInfo(ctx, repository, prNumber)
	if err != nil {
		return err
	}

	resolvedBase, resolvedHead, err := environment.ResolveBaseHead(ctx, prInfo.BaseRef, prInfo.HeadRef)
	if err != nil {
		return err
	}

	recipe, err := c.recipeLoader.Load(ctx, environment, resolvedHead)
	if err != nil {
		return err
	}

	request := usecase.ReplyCommentRequest{
		Repository:          prInfo.Repository,
		RepoURL:             repoURL,
		ChangeRequestNumber: prNumber,
		Title:               prInfo.Title,
		Description:         prInfo.Description,
		Base:                resolvedBase,
		Head:                resolvedHead,
		Publish:             params.Publish,
		Environment:         environment,
		Recipe:              recipe,
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

	reviewComment, reviewErr := vcsClient.GetReviewComment(ctx, prInfo.Repository, prNumber, commentID)
	if reviewErr == nil && reviewComment.ID > 0 {
		request.CommentKind = domain.CommentKindReview
		request.Question = text.StripTrigger(reviewComment.Body, c.triggerName)
		thread, err := buildReviewThread(ctx, vcsClient, prInfo.Repository, prNumber, commentID)
		if err != nil {
			return err
		}
		request.Thread = thread
		_, err = replyCommentUseCase.Execute(ctx, request)
		return err
	}

	issueComment, issueErr := vcsClient.GetIssueComment(ctx, prInfo.Repository, prNumber, commentID)
	if issueErr != nil || issueComment.ID <= 0 {
		if reviewErr != nil {
			return fmt.Errorf("failed to resolve comment: %v", reviewErr)
		}
		return fmt.Errorf("failed to resolve comment: %v", issueErr)
	}
	request.CommentKind = domain.CommentKindIssue
	request.Question = text.StripTrigger(issueComment.Body, c.triggerName)
	thread, err := buildIssueThread(ctx, vcsClient, prInfo.Repository, prNumber, commentID, prInfo)
	if err != nil {
		return err
	}
	request.Thread = thread
	_, err = replyCommentUseCase.Execute(ctx, request)
	return err
}
