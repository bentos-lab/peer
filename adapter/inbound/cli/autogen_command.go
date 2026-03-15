package cli

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	codeenv "bentos-backend/adapter/outbound/codeenv"
	"bentos-backend/shared/logger/stdlogger"
	sharedlogging "bentos-backend/shared/logging"
	"bentos-backend/usecase"
	uccontracts "bentos-backend/usecase/contracts"
)

// AutogenCommand runs autogen flow with the shared autogen usecase.
type AutogenCommand struct {
	autogenUseCaseBuilder AutogenUseCaseBuilder
	githubClient          GitHubClient
	envFactory            uccontracts.CodeEnvironmentFactory
	recipeLoader          usecase.CustomRecipeLoader
	logger                usecase.Logger
}

// AutogenUseCaseBuilder builds an autogen usecase for a specific repo.
type AutogenUseCaseBuilder func(repoURL string) (usecase.AutogenUseCase, error)

// AutogenRunParams contains already-parsed CLI autogen parameters.
type AutogenRunParams struct {
	VCSProvider   string
	Repo          string
	ChangeRequest string
	Base          string
	Head          string
	Publish       bool
	Docs          *bool
	Tests         *bool
}

// NewAutogenCommand creates a new CLI command for autogen.
func NewAutogenCommand(autogenUseCaseBuilder AutogenUseCaseBuilder, githubClient GitHubClient, envFactory uccontracts.CodeEnvironmentFactory, recipeLoader usecase.CustomRecipeLoader, logger usecase.Logger) *AutogenCommand {
	if logger == nil {
		logger = stdlogger.Nop()
	}
	return &AutogenCommand{
		autogenUseCaseBuilder: autogenUseCaseBuilder,
		githubClient:          githubClient,
		envFactory:            envFactory,
		recipeLoader:          recipeLoader,
		logger:                logger,
	}
}

// Run executes the CLI autogen flow.
func (c *AutogenCommand) Run(ctx context.Context, params AutogenRunParams) error {
	if c.autogenUseCaseBuilder == nil {
		return errors.New("autogen usecase is not configured")
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

	provider := strings.TrimSpace(strings.ToLower(params.VCSProvider))
	if provider == "" {
		provider = "github"
	}
	if provider != "github" {
		return fmt.Errorf("unsupported vcs provider: %s", provider)
	}

	if strings.TrimSpace(params.ChangeRequest) != "" && (strings.TrimSpace(params.Base) != "" || strings.TrimSpace(params.Head) != "") {
		return errors.New("--change-request cannot be used with --base or --head")
	}
	if params.Publish && strings.TrimSpace(params.ChangeRequest) == "" {
		return errors.New("--publish requires --change-request")
	}

	repository, repoURL, buildRepoURL, err := normalizeRepo(params.Repo)
	if err != nil {
		return err
	}
	repoProvided := strings.TrimSpace(params.Repo) != ""
	repository, err = c.githubClient.ResolveRepository(ctx, repository)
	if err != nil {
		return err
	}

	base := strings.TrimSpace(params.Base)
	head := strings.TrimSpace(params.Head)
	if head == "" {
		if repoProvided {
			head = "HEAD"
		} else {
			head = "@staged"
		}
	}
	if base == "" && head != "" {
		base = "HEAD"
	}

	prNumber := 0
	headBranch := ""
	title := ""
	description := ""
	if strings.TrimSpace(params.ChangeRequest) != "" {
		parsed, parseErr := strconv.Atoi(strings.TrimSpace(params.ChangeRequest))
		if parseErr != nil || parsed <= 0 {
			return fmt.Errorf("--change-request must be a positive integer")
		}
		prNumber = parsed
		prInfo, infoErr := c.githubClient.GetPullRequestInfo(ctx, repository, prNumber)
		if infoErr != nil {
			return infoErr
		}
		repository = prInfo.Repository
		if repoProvided && buildRepoURL != nil {
			repoURL = buildRepoURL(prInfo.Repository)
		}
		base = prInfo.BaseRef
		head = prInfo.HeadRef
		headBranch = prInfo.HeadRefName
		title = prInfo.Title
		description = prInfo.Description
	}
	if repoProvided && isWorkspaceHeadToken(head) {
		return fmt.Errorf("--head %s requires local workspace mode; omit --repo", head)
	}

	environment, cleanup, err := codeenv.NewEnvironment(ctx, c.envFactory, repoURL)
	if err != nil {
		return err
	}
	defer func() {
		if cleanupErr := cleanup(ctx); cleanupErr != nil {
			c.logger.Warnf("Failed to cleanup code environment: %v", cleanupErr)
		}
	}()

	headRef := strings.TrimSpace(head)
	if headRef == "" {
		headRef = "HEAD"
	}
	recipe, err := c.recipeLoader.Load(ctx, environment, headRef)
	if err != nil {
		return err
	}

	autogenUseCase, err := c.autogenUseCaseBuilder(repoURL)
	if err != nil {
		return err
	}

	effectiveDocs := ResolveBool(params.Docs, recipe.AutogenDocs, false)
	effectiveTests := ResolveBool(params.Tests, recipe.AutogenTests, false)

	request := usecase.AutogenRequest{
		Input:       domainChangeRequestInputForAutogen(repository, prNumber, repoURL, base, head, title, description),
		Docs:        effectiveDocs,
		Tests:       effectiveTests,
		Publish:     params.Publish,
		HeadBranch:  headBranch,
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
