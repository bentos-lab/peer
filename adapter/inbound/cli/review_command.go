package cli

import (
	"context"
	"errors"
	"time"

	codeenv "bentos-backend/adapter/outbound/codeenv"
	githubvcs "bentos-backend/adapter/outbound/vcs/github"
	"bentos-backend/config"
	"bentos-backend/shared/logger/stdlogger"
	sharedlogging "bentos-backend/shared/logging"
	"bentos-backend/usecase"
	uccontracts "bentos-backend/usecase/contracts"
)

// GitHubClient resolves repository and pull-request metadata.
type GitHubClient interface {
	ResolveRepository(ctx context.Context, repository string) (string, error)
	GetPullRequestInfo(ctx context.Context, repository string, pullRequestNumber int) (githubvcs.PullRequestInfo, error)
	GetIssue(ctx context.Context, repository string, issueNumber int) (githubvcs.Issue, error)
	ListIssueComments(ctx context.Context, repository string, pullRequestNumber int) ([]githubvcs.IssueComment, error)
}

// ReviewUseCaseBuilder builds a review usecase for a specific repo.
type ReviewUseCaseBuilder func(repoURL string) (usecase.ReviewUseCase, error)

// ReviewCommand runs autogit review flow with the shared review usecase.
type ReviewCommand struct {
	reviewUseCaseBuilder ReviewUseCaseBuilder
	githubClient         GitHubClient
	envFactory           uccontracts.CodeEnvironmentFactory
	recipeLoader         usecase.CustomRecipeLoader
	logger               usecase.Logger
}

// ReviewParams contains already-parsed CLI autogit parameters.
type ReviewParams struct {
	VCSProvider   string
	Repo          string
	ChangeRequest string
	Base          string
	Head          string
	Publish       bool
	Suggest       *bool
}

type repoURLBuilder func(repository string) string

// NewReviewCommand creates a new CLI command for autogit reviews.
func NewReviewCommand(reviewUseCaseBuilder ReviewUseCaseBuilder, githubClient GitHubClient, envFactory uccontracts.CodeEnvironmentFactory, recipeLoader usecase.CustomRecipeLoader, logger usecase.Logger) *ReviewCommand {
	if logger == nil {
		logger = stdlogger.Nop()
	}
	return &ReviewCommand{
		reviewUseCaseBuilder: reviewUseCaseBuilder,
		githubClient:         githubClient,
		envFactory:           envFactory,
		recipeLoader:         recipeLoader,
		logger:               logger,
	}
}

// Run executes the CLI review flow.
func (c *ReviewCommand) Run(ctx context.Context, cfg config.Config, params ReviewParams) error {
	if c.reviewUseCaseBuilder == nil {
		return errors.New("review usecase is not configured")
	}
	if c.githubClient == nil {
		return errors.New("github client is not configured")
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

	resolution, err := resolveChangeRequestParams(ctx, c.githubClient, ChangeRequestParams{
		VCSProvider:    params.VCSProvider,
		Repo:           params.Repo,
		ChangeRequest:  params.ChangeRequest,
		Base:           params.Base,
		Head:           params.Head,
		Publish:        params.Publish,
		IssueAlignment: false,
	})
	if err != nil {
		return err
	}

	environment, cleanup, err := codeenv.NewEnvironment(ctx, c.envFactory, resolution.RepoURL)
	if err != nil {
		return err
	}
	defer func() {
		if cleanupErr := cleanup(ctx); cleanupErr != nil {
			c.logger.Warnf("Failed to cleanup code environment: %v", cleanupErr)
		}
	}()

	headRef := resolution.Head
	recipe, err := c.recipeLoader.Load(ctx, environment, headRef)
	if err != nil {
		return err
	}

	reviewUseCase, err := c.reviewUseCaseBuilder(resolution.RepoURL)
	if err != nil {
		return err
	}

	request := usecase.ReviewRequest{
		Input: domainChangeRequestInput(
			resolution.Repository,
			resolution.ChangeRequestNumber,
			resolution.RepoURL,
			resolution.Base,
			resolution.Head,
			resolution.Title,
			resolution.Description,
			map[string]string{},
		),
		Suggestions: ResolveBool(params.Suggest, recipe.ReviewSuggestions, cfg.Review.SuggestedChangesEnabled),
		Environment: environment,
		Recipe:      recipe,
	}
	if !params.Publish {
		request.Input.Target.ChangeRequestNumber = 0
	}

	startedAt := time.Now()
	c.logger.Infof("CLI review started.")
	c.logger.Debugf("Repository is %q and change request number is %d.", request.Input.Target.Repository, request.Input.Target.ChangeRequestNumber)
	sharedlogging.LogInputSnapshot(c.logger, "cli", "", request)

	_, err = reviewUseCase.Execute(ctx, request)
	if err != nil {
		c.logger.Errorf("CLI review failed.")
		c.logger.Debugf("The review target was repository %q and change request %d.", request.Input.Target.Repository, request.Input.Target.ChangeRequestNumber)
		c.logger.Debugf("The CLI review ran for %d ms before failing.", time.Since(startedAt).Milliseconds())
		c.logger.Debugf("Failure details: %v.", err)
		return err
	}

	c.logger.Infof("CLI review completed.")
	c.logger.Debugf("The review target was repository %q and change request %d.", request.Input.Target.Repository, request.Input.Target.ChangeRequestNumber)
	c.logger.Debugf("The CLI review completed in %d ms.", time.Since(startedAt).Milliseconds())
	return nil
}
