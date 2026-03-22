package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bentos-lab/peer/config"
	"github.com/bentos-lab/peer/domain"
	sharedcli "github.com/bentos-lab/peer/shared/cli"
	"github.com/bentos-lab/peer/shared/logger/stdlogger"
	sharedlogging "github.com/bentos-lab/peer/shared/logging"
	"github.com/bentos-lab/peer/usecase"
	uccontracts "github.com/bentos-lab/peer/usecase/contracts"
)

// AutogenCommand runs autogen flow with the shared autogen usecase.
type AutogenCommand struct {
	autogenUseCaseBuilder AutogenUseCaseBuilder
	vcsResolver           VCSClientResolver
	envFactory            uccontracts.CodeEnvironmentFactory
	recipeLoader          usecase.CustomRecipeLoader
	logger                usecase.Logger
}

// AutogenUseCaseBuilder builds an autogen usecase for a specific repo.
type AutogenUseCaseBuilder func(repoURL string) (usecase.AutogenUseCase, error)

// AutogenRunParams contains already-parsed CLI autogen parameters.
type AutogenRunParams struct {
	VCSProvider   string
	VCSHost       string
	Repo          string
	ChangeRequest string
	Base          string
	Head          string
	Publish       bool
	Docs          *bool
	Tests         *bool
}

// NewAutogenCommand creates a new CLI command for autogen.
func NewAutogenCommand(autogenUseCaseBuilder AutogenUseCaseBuilder, vcsResolver VCSClientResolver, envFactory uccontracts.CodeEnvironmentFactory, recipeLoader usecase.CustomRecipeLoader, logger usecase.Logger) *AutogenCommand {
	if logger == nil {
		logger = stdlogger.Nop()
	}
	return &AutogenCommand{
		autogenUseCaseBuilder: autogenUseCaseBuilder,
		vcsResolver:           vcsResolver,
		envFactory:            envFactory,
		recipeLoader:          recipeLoader,
		logger:                logger,
	}
}

// Run executes the CLI autogen flow.
func (c *AutogenCommand) Run(ctx context.Context, cfg config.Config, params AutogenRunParams) error {
	if c.autogenUseCaseBuilder == nil {
		return errors.New("autogen usecase is not configured")
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

	headRef := strings.TrimSpace(resolution.Head)
	if headRef == "" {
		headRef = "HEAD"
	}
	recipe, err := c.recipeLoader.Load(ctx, environment, headRef)
	if err != nil {
		return err
	}

	autogenUseCase, err := c.autogenUseCaseBuilder(resolution.RepoURL)
	if err != nil {
		return err
	}

	effectiveDocs := sharedcli.ResolveBool(params.Docs, recipe.AutogenDocs, cfg.Autogen.DocsEnabled)
	effectiveTests := sharedcli.ResolveBool(params.Tests, recipe.AutogenTests, cfg.Autogen.TestsEnabled)

	request := usecase.AutogenRequest{
		Input:       domainChangeRequestInputForAutogen(resolution.Repository, resolution.ChangeRequestNumber, resolution.RepoURL, resolution.Base, resolution.Head, resolution.Title, resolution.Description),
		Docs:        effectiveDocs,
		Tests:       effectiveTests,
		Publish:     params.Publish,
		HeadBranch:  strings.TrimSpace(resolution.HeadRefName),
		Environment: environment,
		Recipe:      recipe,
	}
	if !params.Publish {
		request.Input.Target.ChangeRequestNumber = 0
		request.Publish = false
		request.HeadBranch = ""
	}

	startedAt := time.Now()
	c.logger.Infof("CLI autogen started.")
	c.logger.Debugf("Repository is %q and change request number is %d.", request.Input.Target.Repository, request.Input.Target.ChangeRequestNumber)
	sharedlogging.LogInputSnapshot(c.logger, "cli", "", request)

	_, err = autogenUseCase.Execute(ctx, request)
	if err != nil {
		c.logger.Errorf("CLI autogen failed.")
		c.logger.Debugf("The autogen target was repository %q and change request %d.", request.Input.Target.Repository, request.Input.Target.ChangeRequestNumber)
		c.logger.Debugf("The CLI autogen ran for %d ms before failing.", time.Since(startedAt).Milliseconds())
		c.logger.Debugf("Failure details: %v.", err)
		return err
	}

	c.logger.Infof("CLI autogen completed.")
	c.logger.Debugf("The autogen target was repository %q and change request %d.", request.Input.Target.Repository, request.Input.Target.ChangeRequestNumber)
	c.logger.Debugf("The CLI autogen completed in %d ms.", time.Since(startedAt).Milliseconds())
	return nil
}
