package cli

import (
	"context"
	"errors"
	"time"

	codeenv "bentos-backend/adapter/outbound/codeenv"
	"bentos-backend/shared/logger/stdlogger"
	sharedlogging "bentos-backend/shared/logging"
	"bentos-backend/usecase"
	uccontracts "bentos-backend/usecase/contracts"
)

// OverviewCommand runs autogit overview flow with the shared change request usecase.
type OverviewCommand struct {
	changeRequestUseCaseBuilder ChangeRequestUseCaseBuilder
	githubClient                GitHubClient
	envFactory                  uccontracts.CodeEnvironmentFactory
	recipeLoader                usecase.CustomRecipeLoader
	logger                      usecase.Logger
}

// OverviewParams contains already-parsed CLI autogit parameters for overviews.
type OverviewParams struct {
	VCSProvider    string
	Repo           string
	ChangeRequest  string
	Base           string
	Head           string
	Publish        bool
	IssueAlignment bool
}

// NewOverviewCommand creates a new CLI command for autogit overviews.
func NewOverviewCommand(changeRequestUseCaseBuilder ChangeRequestUseCaseBuilder, githubClient GitHubClient, envFactory uccontracts.CodeEnvironmentFactory, recipeLoader usecase.CustomRecipeLoader, logger usecase.Logger) *OverviewCommand {
	if logger == nil {
		logger = stdlogger.Nop()
	}
	return &OverviewCommand{
		changeRequestUseCaseBuilder: changeRequestUseCaseBuilder,
		githubClient:                githubClient,
		envFactory:                  envFactory,
		recipeLoader:                recipeLoader,
		logger:                      logger,
	}
}

// Run executes the CLI overview flow.
func (c *OverviewCommand) Run(ctx context.Context, params OverviewParams) error {
	if c.changeRequestUseCaseBuilder == nil {
		return errors.New("change request usecase is not configured")
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
		IssueAlignment: params.IssueAlignment,
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

	changeRequestUseCase, err := c.changeRequestUseCaseBuilder(resolution.RepoURL)
	if err != nil {
		return err
	}

	request := usecase.ChangeRequestRequest{
		Repository:          resolution.Repository,
		RepoURL:             resolution.RepoURL,
		ChangeRequestNumber: resolution.ChangeRequestNumber,
		Title:               resolution.Title,
		Description:         resolution.Description,
		Base:                resolution.Base,
		Head:                resolution.Head,
		EnableReview:        false,
		EnableOverview:      true,
		EnableSuggestions:   false,
		OverviewIssueAlignment: usecase.OverviewIssueAlignmentInput{
			Candidates: resolution.IssueCandidates,
		},
		Environment: environment,
		Recipe:      recipe,
	}
	if !params.Publish {
		request.ChangeRequestNumber = 0
	}

	startedAt := time.Now()
	c.logger.Infof("CLI overview started.")
	c.logger.Debugf("Repository is %q and change request number is %d.", request.Repository, request.ChangeRequestNumber)
	sharedlogging.LogInputSnapshot(c.logger, "cli", "", request)

	_, err = changeRequestUseCase.Execute(ctx, request)
	if err != nil {
		c.logger.Errorf("CLI overview failed.")
		c.logger.Debugf("The overview target was repository %q and change request %d.", request.Repository, request.ChangeRequestNumber)
		c.logger.Debugf("The CLI overview ran for %d ms before failing.", time.Since(startedAt).Milliseconds())
		c.logger.Debugf("Failure details: %v.", err)
		return err
	}

	c.logger.Infof("CLI overview completed.")
	c.logger.Debugf("The overview target was repository %q and change request %d.", request.Repository, request.ChangeRequestNumber)
	c.logger.Debugf("The CLI overview completed in %d ms.", time.Since(startedAt).Milliseconds())
	return nil
}
