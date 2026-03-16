package cli

import (
	"context"
	"errors"

	"bentos-backend/shared/toolinstall"
)

// ToolInstaller defines installation capabilities for CLI dependencies.
type ToolInstaller interface {
	EnsureGhInstalled(ctx context.Context) error
	EnsureGhAuthenticated(ctx context.Context) error
	EnsureGlabInstalled(ctx context.Context) error
	EnsureGlabAuthenticated(ctx context.Context) error
	EnsureOpencodeInstalled(ctx context.Context) error
}

// InstallCommand installs required CLI dependencies.
type InstallCommand struct {
	installer ToolInstaller
}

// NewInstallCommand creates a new install command with defaults.
func NewInstallCommand() *InstallCommand {
	return &InstallCommand{installer: toolinstall.NewInstaller(toolinstall.Config{})}
}

// InstallGh installs GitHub CLI and optionally logs in.
func (c *InstallCommand) InstallGh(ctx context.Context, login bool) error {
	installer := c.resolveInstaller()
	if err := installer.EnsureGhInstalled(ctx); err != nil {
		return err
	}
	if !login {
		return nil
	}
	return installer.EnsureGhAuthenticated(ctx)
}

// InstallGlab installs GitLab CLI and optionally logs in.
func (c *InstallCommand) InstallGlab(ctx context.Context, login bool) error {
	installer := c.resolveInstaller()
	if err := installer.EnsureGlabInstalled(ctx); err != nil {
		return err
	}
	if !login {
		return nil
	}
	return installer.EnsureGlabAuthenticated(ctx)
}

// InstallOpencode installs OpenCode (opencode).
func (c *InstallCommand) InstallOpencode(ctx context.Context) error {
	installer := c.resolveInstaller()
	return installer.EnsureOpencodeInstalled(ctx)
}

// InstallQuickstart installs gh (with login) and opencode.
func (c *InstallCommand) InstallQuickstart(ctx context.Context) error {
	if err := c.InstallGh(ctx, true); err != nil {
		return err
	}
	return c.InstallOpencode(ctx)
}

type missingInstaller struct{}

func (missingInstaller) EnsureGhInstalled(context.Context) error {
	return errors.New("install command is not configured")
}

func (missingInstaller) EnsureGhAuthenticated(context.Context) error {
	return errors.New("install command is not configured")
}

func (missingInstaller) EnsureGlabInstalled(context.Context) error {
	return errors.New("install command is not configured")
}

func (missingInstaller) EnsureGlabAuthenticated(context.Context) error {
	return errors.New("install command is not configured")
}

func (missingInstaller) EnsureOpencodeInstalled(context.Context) error {
	return errors.New("install command is not configured")
}
