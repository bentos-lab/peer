package cli

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	inboundlogging "bentos-backend/adapter/inbound/logging"
	"bentos-backend/domain"
	"bentos-backend/shared/logger/stdlogger"
	"bentos-backend/usecase"
)

// AutogenCommand runs autogen flow with the shared autogen usecase.
type AutogenCommand struct {
	autogenUseCase usecase.AutogenUseCase
	githubClient   GitHubClient
	logger         usecase.Logger
}

// AutogenRunParams contains already-parsed CLI autogen parameters.
type AutogenRunParams struct {
	VCSProvider   string
	Repo          string
	ChangeRequest string
	Base          string
	Head          string
	Publish       bool
	Docs          bool
	Tests         bool
}

// NewAutogenCommand creates a new CLI command for autogen.
func NewAutogenCommand(autogenUseCase usecase.AutogenUseCase, githubClient GitHubClient, logger usecase.Logger) *AutogenCommand {
	if logger == nil {
		logger = stdlogger.Nop()
	}
	return &AutogenCommand{
		autogenUseCase: autogenUseCase,
		githubClient:   githubClient,
		logger:         logger,
	}
}

// Run executes the CLI autogen flow.
func (c *AutogenCommand) Run(ctx context.Context, params AutogenRunParams) error {
	if c.autogenUseCase == nil {
		return errors.New("autogen usecase is not configured")
	}
	if c.githubClient == nil {
		return errors.New("github client is not configured")
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

	request := usecase.AutogenRequest{
		Input:      domainChangeRequestInputForAutogen(repository, prNumber, repoURL, base, head, title, description),
		Docs:       params.Docs,
		Tests:      params.Tests,
		Publish:    params.Publish,
		HeadBranch: headBranch,
	}
	if !params.Publish {
		request.Input.Target.ChangeRequestNumber = 0
		request.Publish = false
		request.HeadBranch = ""
	}

	startedAt := time.Now()
	c.logger.Infof("CLI autogen started.")
	c.logger.Debugf("Repository is %q and change request number is %d.", request.Input.Target.Repository, request.Input.Target.ChangeRequestNumber)
	inboundlogging.LogChangeRequestInputSnapshot(c.logger, "cli", "", usecase.ChangeRequestRequest{
		Repository:          request.Input.Target.Repository,
		RepoURL:             request.Input.RepoURL,
		ChangeRequestNumber: request.Input.Target.ChangeRequestNumber,
		Title:               request.Input.Title,
		Description:         request.Input.Description,
		Base:                request.Input.Base,
		Head:                request.Input.Head,
		EnableOverview:      false,
		EnableSuggestions:   false,
		OverviewExplicit:    false,
		SuggestionsExplicit: false,
		Metadata:            request.Input.Metadata,
	})

	_, err = c.autogenUseCase.Execute(ctx, request)
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

func domainChangeRequestInputForAutogen(repository string, prNumber int, repoURL string, base string, head string, title string, description string) domain.ChangeRequestInput {
	return domain.ChangeRequestInput{
		Target:      domain.ChangeRequestTarget{Repository: repository, ChangeRequestNumber: prNumber},
		RepoURL:     repoURL,
		Base:        base,
		Head:        head,
		Title:       title,
		Description: description,
		Language:    "English",
		Metadata:    map[string]string{},
	}
}
