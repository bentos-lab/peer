package cli

import (
	"context"
	"errors"
	"strings"

	"github.com/bentos-lab/peer/shared/toolinstall"
)

// UpdateRunner defines the update behavior.
type UpdateRunner interface {
	LatestVersion(context.Context) (string, error)
	Update(context.Context) (toolinstall.UpdateResult, error)
}

// UpdateOutcome captures the update command outcome.
type UpdateOutcome struct {
	UpToDate bool
	Result   toolinstall.UpdateResult
}

// UpdateCommand updates the peer CLI to the latest stable release.
type UpdateCommand struct {
	updater        UpdateRunner
	currentVersion string
}

// NewUpdateCommand creates a new update command with defaults.
func NewUpdateCommand(currentVersion string) *UpdateCommand {
	return &UpdateCommand{
		updater:        toolinstall.NewUpdater(nil),
		currentVersion: currentVersion,
	}
}

// Run performs the update and returns the result.
func (c *UpdateCommand) Run(ctx context.Context) (UpdateOutcome, error) {
	updater, err := c.resolveUpdater()
	if err != nil {
		return UpdateOutcome{}, err
	}
	latest, err := updater.LatestVersion(ctx)
	if err != nil {
		return UpdateOutcome{}, err
	}
	if normalizeVersion(latest) == normalizeVersion(c.currentVersion) {
		return UpdateOutcome{
			UpToDate: true,
			Result: toolinstall.UpdateResult{
				Version: latest,
			},
		}, nil
	}
	result, err := updater.Update(ctx)
	if err != nil {
		return UpdateOutcome{}, err
	}
	return UpdateOutcome{Result: result}, nil
}

var errUpdateNotConfigured = errors.New("update command is not configured")

func (c *UpdateCommand) resolveUpdater() (UpdateRunner, error) {
	if c == nil {
		return nil, errUpdateNotConfigured
	}
	if c.updater == nil {
		c.updater = toolinstall.NewUpdater(nil)
	}
	return c.updater, nil
}

func normalizeVersion(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "v")
	value = strings.TrimPrefix(value, "V")
	if idx := strings.IndexAny(value, "-+"); idx != -1 {
		value = value[:idx]
	}
	return value
}
