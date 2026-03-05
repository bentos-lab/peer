package cli

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	cliinput "bentos-backend/adapter/outbound/input/cli"
	"bentos-backend/domain"
	"bentos-backend/usecase"
)

// Command runs local reviews with the shared review usecase.
type Command struct {
	reviewer     usecase.ReviewUseCase
	providerName domain.ReviewInputProvider
	logger       usecase.Logger
}

// RunParams contains already-parsed CLI review parameters.
type RunParams struct {
	ChangedFiles     string
	IncludeUnstaged  bool
	IncludeUntracked bool
	PRNumber         int
}

// NewLocalCommand creates a new CLI command for local reviews.
func NewLocalCommand(reviewer usecase.ReviewUseCase, logger usecase.Logger) *Command {
	if logger == nil {
		logger = usecase.NopLogger
	}
	return &Command{
		reviewer:     reviewer,
		providerName: domain.ReviewInputProviderLocal,
		logger:       logger,
	}
}

// NewGitHubPRCommand creates a new CLI command for GitHub pull request reviews.
func NewGitHubPRCommand(reviewer usecase.ReviewUseCase, logger usecase.Logger) *Command {
	if logger == nil {
		logger = usecase.NopLogger
	}
	return &Command{
		reviewer:     reviewer,
		providerName: domain.ReviewInputProviderGitHub,
		logger:       logger,
	}
}

// Run executes the CLI review flow.
func (c *Command) Run(ctx context.Context, params RunParams) error {
	if c.reviewer == nil {
		return errors.New("review usecase is not configured")
	}
	if c.logger == nil {
		c.logger = usecase.NopLogger
	}

	repository := "local/repo"
	metadata := map[string]string{
		cliinput.MetadataKeyChangedFiles:         strings.TrimSpace(params.ChangedFiles),
		cliinput.MetadataKeyAutoIncludeAll:       strconv.FormatBool(params.IncludeUnstaged),
		cliinput.MetadataKeyAutoIncludeUntracked: strconv.FormatBool(params.IncludeUntracked),
	}
	switch c.providerName {
	case domain.ReviewInputProviderLocal:
		// Keep metadata and repository for local review.
	case domain.ReviewInputProviderGitHub:
		repository = ""
		metadata = map[string]string{}
	default:
		return errors.New("review provider is not configured")
	}

	request := usecase.ReviewRequest{
		Repository:          repository,
		ChangeRequestNumber: params.PRNumber,
		Metadata:            metadata,
	}

	startedAt := time.Now()
	c.logger.Infof("CLI review started.")
	c.logger.Debugf("Using %s provider.", string(c.providerName))
	c.logger.Debugf("Repository is %q and change request number is %d.", request.Repository, request.ChangeRequestNumber)

	_, err := c.reviewer.Execute(ctx, request)
	if err != nil {
		c.logger.Errorf("CLI review failed.")
		c.logger.Debugf("The review used %s provider for repository %q and change request %d.", string(c.providerName), request.Repository, request.ChangeRequestNumber)
		c.logger.Debugf("The CLI review ran for %d ms before failing.", time.Since(startedAt).Milliseconds())
		c.logger.Debugf("Failure details: %v.", err)
		return err
	}

	c.logger.Infof("CLI review completed.")
	c.logger.Debugf("The review used %s provider for repository %q and change request %d.", string(c.providerName), request.Repository, request.ChangeRequestNumber)
	c.logger.Debugf("The CLI review completed in %d ms.", time.Since(startedAt).Milliseconds())
	return nil
}
