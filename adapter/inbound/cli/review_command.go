package cli

import (
	"context"
	"errors"
	"time"

	inboundlogging "bentos-backend/adapter/inbound/logging"
	githubvcs "bentos-backend/adapter/outbound/vcs/github"
	"bentos-backend/shared/logger/stdlogger"
	"bentos-backend/usecase"
)

// GitHubClient resolves repository and pull-request metadata.
type GitHubClient interface {
	ResolveRepository(ctx context.Context, repository string) (string, error)
	GetPullRequestInfo(ctx context.Context, repository string, pullRequestNumber int) (githubvcs.PullRequestInfo, error)
}

// ChangeRequestUseCaseBuilder builds a change request usecase for a specific repo.
type ChangeRequestUseCaseBuilder func(repoURL string) (usecase.ChangeRequestUseCase, error)

// ReviewCommand runs autogit review flow with the shared change request usecase.
type ReviewCommand struct {
	changeRequestUseCaseBuilder ChangeRequestUseCaseBuilder
	githubClient                GitHubClient
	logger                      usecase.Logger
}

// ReviewParams contains already-parsed CLI autogit parameters.
type ReviewParams struct {
	VCSProvider     string
	Repo            string
	ChangeRequest   string
	Base            string
	Head            string
	Comment         bool
	Suggest         bool
	SuggestExplicit bool
}

type repoURLBuilder func(repository string) string

// NewReviewCommand creates a new CLI command for autogit reviews.
func NewReviewCommand(changeRequestUseCaseBuilder ChangeRequestUseCaseBuilder, githubClient GitHubClient, logger usecase.Logger) *ReviewCommand {
	if logger == nil {
		logger = stdlogger.Nop()
	}
	return &ReviewCommand{
		changeRequestUseCaseBuilder: changeRequestUseCaseBuilder,
		githubClient:                githubClient,
		logger:                      logger,
	}
}

// Run executes the CLI review flow.
func (c *ReviewCommand) Run(ctx context.Context, params ReviewParams) error {
	if c.changeRequestUseCaseBuilder == nil {
		return errors.New("change request usecase is not configured")
	}
	if c.githubClient == nil {
		return errors.New("github client is not configured")
	}
	if c.logger == nil {
		c.logger = stdlogger.Nop()
	}

	resolution, err := resolveChangeRequestParams(ctx, c.githubClient, ChangeRequestParams{
		VCSProvider:   params.VCSProvider,
		Repo:          params.Repo,
		ChangeRequest: params.ChangeRequest,
		Base:          params.Base,
		Head:          params.Head,
		Comment:       params.Comment,
	})
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
		EnableReview:        true,
		EnableOverview:      false,
		EnableSuggestions:   params.Suggest,
		ReviewExplicit:      true,
		OverviewExplicit:    false,
		SuggestionsExplicit: params.SuggestExplicit,
	}
	if !params.Comment {
		request.ChangeRequestNumber = 0
	}

	startedAt := time.Now()
	c.logger.Infof("CLI review started.")
	c.logger.Debugf("Repository is %q and change request number is %d.", request.Repository, request.ChangeRequestNumber)
	inboundlogging.LogChangeRequestInputSnapshot(c.logger, "cli", "", request)

	_, err = changeRequestUseCase.Execute(ctx, request)
	if err != nil {
		c.logger.Errorf("CLI review failed.")
		c.logger.Debugf("The review target was repository %q and change request %d.", request.Repository, request.ChangeRequestNumber)
		c.logger.Debugf("The CLI review ran for %d ms before failing.", time.Since(startedAt).Milliseconds())
		c.logger.Debugf("Failure details: %v.", err)
		return err
	}

	c.logger.Infof("CLI review completed.")
	c.logger.Debugf("The review target was repository %q and change request %d.", request.Repository, request.ChangeRequestNumber)
	c.logger.Debugf("The CLI review completed in %d ms.", time.Since(startedAt).Milliseconds())
	return nil
}
