package cli

import (
	"context"
	"errors"

	"bentos-backend/shared/toolinstall"
)

// UpdateRunner defines the update behavior.
type UpdateRunner interface {
	Update(context.Context) (toolinstall.UpdateResult, error)
}

// UpdateCommand updates the peer CLI to the latest stable release.
type UpdateCommand struct {
	updater UpdateRunner
}

// NewUpdateCommand creates a new update command with defaults.
func NewUpdateCommand() *UpdateCommand {
	return &UpdateCommand{updater: toolinstall.NewUpdater(nil)}
}

// Run performs the update and returns the result.
func (c *UpdateCommand) Run(ctx context.Context) (toolinstall.UpdateResult, error) {
	updater, err := c.resolveUpdater()
	if err != nil {
		return toolinstall.UpdateResult{}, err
	}
	return updater.Update(ctx)
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
