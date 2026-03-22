package cli

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bentos-lab/peer/config"
	"github.com/bentos-lab/peer/domain"
	sharedcli "github.com/bentos-lab/peer/shared/cli"
	"github.com/bentos-lab/peer/shared/logger/stdlogger"
	sharedlogging "github.com/bentos-lab/peer/shared/logging"
	"github.com/bentos-lab/peer/usecase"
	uccontracts "github.com/bentos-lab/peer/usecase/contracts"
)

// OverviewCommand runs peer overview flow with the shared overview usecase.
type OverviewCommand struct {
	overviewUseCaseBuilder OverviewUseCaseBuilder
	vcsResolver            VCSClientResolver
	envFactory             uccontracts.CodeEnvironmentFactory
	recipeLoader           usecase.CustomRecipeLoader
	logger                 usecase.Logger
}

// OverviewParams contains already-parsed CLI peer parameters for overviews.
type OverviewParams struct {
	VCSProvider    string
	VCSHost        string
	Repo           string
	ChangeRequest  string
	Base           string
	Head           string
	Publish        bool
	IssueAlignment *bool
}

// OverviewUseCaseBuilder builds an overview usecase for a specific repo.
type OverviewUseCaseBuilder func(repoURL string) (usecase.OverviewUseCase, error)

// NewOverviewCommand creates a new CLI command for peer overviews.
func NewOverviewCommand(overviewUseCaseBuilder OverviewUseCaseBuilder, vcsResolver VCSClientResolver, envFactory uccontracts.CodeEnvironmentFactory, recipeLoader usecase.CustomRecipeLoader, logger usecase.Logger) *OverviewCommand {
	if logger == nil {
		logger = stdlogger.Nop()
	}
	return &OverviewCommand{
		overviewUseCaseBuilder: overviewUseCaseBuilder,
		vcsResolver:            vcsResolver,
		envFactory:             envFactory,
		recipeLoader:           recipeLoader,
		logger:                 logger,
	}
}

// Run executes the CLI overview flow.
func (c *OverviewCommand) Run(ctx context.Context, cfg config.Config, params OverviewParams) error {
	if c.overviewUseCaseBuilder == nil {
		return errors.New("overview usecase is not configured")
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

	effectiveIssueAlignment := sharedcli.ResolveBool(params.IssueAlignment, nil, cfg.Overview.IssueAlignmentEnabled)
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
		IssueAlignment: effectiveIssueAlignment,
	})
	if err != nil {
		return err
	}

	if c.envFactory == nil {
		return fmt.Errorf("code environment factory is required")
	}
	environment, err := c.envFactory.New(ctx, domain.CodeEnvironmentInitOptions{
		RepoURL: resolution.RepoURL,
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

	headRef := resolution.Head
	recipe, err := c.recipeLoader.Load(ctx, environment, headRef)
	if err != nil {
		return err
	}
	effectiveIssueAlignment = sharedcli.ResolveBool(params.IssueAlignment, recipe.OverviewIssueAlignmentEnabled, cfg.Overview.IssueAlignmentEnabled)
	issueCandidates := resolution.IssueCandidates
	if effectiveIssueAlignment {
		if issueCandidates == nil {
			issueCandidates = resolveIssueCandidates(ctx, normalizeVCSProvider(params.VCSProvider), vcsClient, resolution.Repository, resolution.Description)
		}
	} else {
		issueCandidates = nil
	}

	overviewUseCase, err := c.overviewUseCaseBuilder(resolution.RepoURL)
	if err != nil {
		return err
	}

	request := usecase.OverviewRequest{
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
		IssueAlignment: usecase.OverviewIssueAlignmentInput{Candidates: issueCandidates},
		Environment:    environment,
		Recipe:         recipe,
	}
	if !params.Publish {
		request.Input.Target.ChangeRequestNumber = 0
	}

	startedAt := time.Now()
	c.logger.Infof("CLI overview started.")
	c.logger.Debugf("Repository is %q and change request number is %d.", request.Input.Target.Repository, request.Input.Target.ChangeRequestNumber)
	sharedlogging.LogInputSnapshot(c.logger, "cli", "", request)

	_, err = overviewUseCase.Execute(ctx, request)
	if err != nil {
		c.logger.Errorf("CLI overview failed.")
		c.logger.Debugf("The overview target was repository %q and change request %d.", request.Input.Target.Repository, request.Input.Target.ChangeRequestNumber)
		c.logger.Debugf("The CLI overview ran for %d ms before failing.", time.Since(startedAt).Milliseconds())
		c.logger.Debugf("Failure details: %v.", err)
		return err
	}

	c.logger.Infof("CLI overview completed.")
	c.logger.Debugf("The overview target was repository %q and change request %d.", request.Input.Target.Repository, request.Input.Target.ChangeRequestNumber)
	c.logger.Debugf("The CLI overview completed in %d ms.", time.Since(startedAt).Milliseconds())
	return nil
}
