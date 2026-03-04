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
}

// RunParams contains already-parsed CLI review parameters.
type RunParams struct {
	ChangedFiles     string
	IncludeUnstaged  bool
	IncludeUntracked bool
	PRNumber         int
}

// NewLocalCommand creates a new CLI command for local reviews.
func NewLocalCommand(reviewer usecase.ReviewUseCase) *Command {
	return &Command{
		reviewer:     reviewer,
		providerName: domain.ReviewInputProviderLocal,
	}
}

// NewGitHubPRCommand creates a new CLI command for GitHub pull request reviews.
func NewGitHubPRCommand(reviewer usecase.ReviewUseCase) *Command {
	return &Command{
		reviewer:     reviewer,
		providerName: domain.ReviewInputProviderGitHub,
	}
}

// Run executes the CLI review flow.
func (c *Command) Run(ctx context.Context, params RunParams) error {
	if c.reviewer == nil {
		return errors.New("review usecase is not configured")
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

	_, err := c.reviewer.Execute(ctx, usecase.ReviewRequest{
		ReviewID:            time.Now().UTC().Format(time.RFC3339Nano),
		Repository:          repository,
		ChangeRequestNumber: params.PRNumber,
		Metadata:            metadata,
	})
	return err
}
