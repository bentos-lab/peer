package cli

import (
	"context"
	"errors"

	"bentos-backend/shared/skillupdate"
)

// SkillUpdater defines update behavior for peer skills.
type SkillUpdater interface {
	Update(context.Context, []string) ([]skillupdate.Result, error)
}

// SkillUpdateCommand updates peer skills using embedded assets.
type SkillUpdateCommand struct {
	updater SkillUpdater
}

// NewSkillUpdateCommand creates a new skill update command with defaults.
func NewSkillUpdateCommand() *SkillUpdateCommand {
	return &SkillUpdateCommand{updater: skillupdate.NewUpdater(nil)}
}

// Run performs the skill update and returns the results.
func (c *SkillUpdateCommand) Run(ctx context.Context, paths []string) ([]skillupdate.Result, error) {
	updater, err := c.resolveUpdater()
	if err != nil {
		return nil, err
	}
	return updater.Update(ctx, paths)
}

var errSkillUpdateNotConfigured = errors.New("skill update command is not configured")

func (c *SkillUpdateCommand) resolveUpdater() (SkillUpdater, error) {
	if c == nil {
		return nil, errSkillUpdateNotConfigured
	}
	if c.updater == nil {
		c.updater = skillupdate.NewUpdater(nil)
	}
	return c.updater, nil
}
