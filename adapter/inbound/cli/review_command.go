package cli

import (
	"context"
	"errors"
	"time"

	codeenv "github.com/bentos-lab/peer/adapter/outbound/codeenv"
	"github.com/bentos-lab/peer/config"
	"github.com/bentos-lab/peer/shared/logger/stdlogger"
	sharedlogging "github.com/bentos-lab/peer/shared/logging"
	"github.com/bentos-lab/peer/usecase"
	uccontracts "github.com/bentos-lab/peer/usecase/contracts"
)

// ReviewUseCaseBuilder builds a review usecase for a specific repo.
type ReviewUseCaseBuilder func(repoURL string) (usecase.ReviewUseCase, error)

// ReviewCommand runs peer review flow with the shared review usecase.
type ReviewCommand struct {
	reviewUseCaseBuilder ReviewUseCaseBuilder
	vcsResolver          VCSClientResolver
	envFactory           uccontracts.CodeEnvironmentFactory
	recipeLoader         usecase.CustomRecipeLoader
	logger               usecase.Logger
}

// ReviewParams contains already-parsed CLI peer parameters.
type ReviewParams struct {
	VCSProvider   string
	VCSHost       string
	Repo          string
	ChangeRequest string
	Base          string
	Head          string
	Publish       bool
	Suggest       *bool
}

// NewReviewCommand creates a new CLI command for peer reviews.
func NewReviewCommand(reviewUseCaseBuilder ReviewUseCaseBuilder, vcsResolver VCSClientResolver, envFactory uccontracts.CodeEnvironmentFactory, recipeLoader usecase.CustomRecipeLoader, logger usecase.Logger) *ReviewCommand {
	if logger == nil {
		logger = stdlogger.Nop()
	}
	return &ReviewCommand{
		reviewUseCaseBuilder: reviewUseCaseBuilder,
		vcsResolver:          vcsResolver,
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

	vcsClient, err := c.vcsResolver.Resolve(params.VCSProvider)
	if err != nil {
		return err
	}

	resolution, err := resolveChangeRequestParams(ctx, vcsClient, ChangeRequestParams{
		VCSProvider:    params.VCSProvider,
		VCSHost:        params.VCSHost,
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
