package cli

import (
	"context"
	"strconv"
	"strings"
	"time"

	cliinput "bentos-backend/adapter/outbound/input/cli"
	"bentos-backend/usecase"
)

// Command runs local reviews with the shared review usecase.
type Command struct {
	reviewer usecase.ReviewUseCase
}

// RunParams contains already-parsed CLI review parameters.
type RunParams struct {
	ChangedFiles     string
	IncludeUnstaged  bool
	IncludeUntracked bool
}

// NewCommand creates a new CLI command.
func NewCommand(reviewer usecase.ReviewUseCase) *Command {
	return &Command{reviewer: reviewer}
}

// Run executes the CLI review flow.
func (c *Command) Run(ctx context.Context, params RunParams) error {
	_, err := c.reviewer.Execute(ctx, usecase.ReviewRequest{
		ReviewID:            time.Now().UTC().Format(time.RFC3339Nano),
		Repository:          "local/repo",
		ChangeRequestNumber: 0,
		Metadata: map[string]string{
			cliinput.MetadataKeyChangedFiles:         strings.TrimSpace(params.ChangedFiles),
			cliinput.MetadataKeyAutoIncludeAll:       strconv.FormatBool(params.IncludeUnstaged),
			cliinput.MetadataKeyAutoIncludeUntracked: strconv.FormatBool(params.IncludeUntracked),
		},
	})
	return err
}
