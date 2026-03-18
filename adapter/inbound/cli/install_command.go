package cli

import (
	"context"
	"errors"

	"bentos-backend/shared/toolinstall"
)

// GhInstaller installs GitHub CLI and handles auth.
type GhInstaller interface {
	EnsureGhInstalled(context.Context) error
	EnsureGhAuthenticated(context.Context) error
}

// GlabInstaller installs GitLab CLI and handles auth.
type GlabInstaller interface {
	EnsureGlabInstalled(context.Context) error
	EnsureGlabAuthenticated(context.Context) error
}

// OpencodeInstaller installs OpenCode.
type OpencodeInstaller interface {
	EnsureOpencodeInstalled(context.Context) error
}

// GitInstaller installs Git.
type GitInstaller interface {
	EnsureGitInstalled(context.Context) error
}

// InstallCommand installs required CLI dependencies.
type InstallCommand struct {
	gh       GhInstaller
	glab     GlabInstaller
	opencode OpencodeInstaller
	git      GitInstaller
}

// NewInstallCommand creates a new install command with defaults.
func NewInstallCommand() *InstallCommand {
	return &InstallCommand{
		gh:       toolinstall.NewGhInstaller(nil),
		glab:     toolinstall.NewGlabInstaller(nil),
		opencode: toolinstall.NewOpencodeInstaller(nil),
		git:      toolinstall.NewGitInstaller(nil),
	}
}

// InstallGh installs GitHub CLI and optionally logs in.
func (c *InstallCommand) InstallGh(ctx context.Context, login bool) error {
	installer, err := c.ghInstaller()
	if err != nil {
		return err
	}
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
	installer, err := c.glabInstaller()
	if err != nil {
		return err
	}
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
	installer, err := c.opencodeInstaller()
	if err != nil {
		return err
	}
	return installer.EnsureOpencodeInstalled(ctx)
}

// InstallGit installs Git.
func (c *InstallCommand) InstallGit(ctx context.Context) error {
	installer, err := c.gitInstaller()
	if err != nil {
		return err
	}
	return installer.EnsureGitInstalled(ctx)
}

// InstallQuickstart installs gh (with login) and opencode.
func (c *InstallCommand) InstallQuickstart(ctx context.Context) error {
	if err := c.InstallGh(ctx, true); err != nil {
		return err
	}
	return c.InstallOpencode(ctx)
}

var errInstallNotConfigured = errors.New("install command is not configured")

func (c *InstallCommand) ghInstaller() (GhInstaller, error) {
	if c == nil {
		return nil, errInstallNotConfigured
	}
	if c.gh == nil {
		c.gh = toolinstall.NewGhInstaller(nil)
	}
	return c.gh, nil
}

func (c *InstallCommand) glabInstaller() (GlabInstaller, error) {
	if c == nil {
		return nil, errInstallNotConfigured
	}
	if c.glab == nil {
		c.glab = toolinstall.NewGlabInstaller(nil)
	}
	return c.glab, nil
}

func (c *InstallCommand) opencodeInstaller() (OpencodeInstaller, error) {
	if c == nil {
		return nil, errInstallNotConfigured
	}
	if c.opencode == nil {
		c.opencode = toolinstall.NewOpencodeInstaller(nil)
	}
	return c.opencode, nil
}

func (c *InstallCommand) gitInstaller() (GitInstaller, error) {
	if c == nil {
		return nil, errInstallNotConfigured
	}
	if c.git == nil {
		c.git = toolinstall.NewGitInstaller(nil)
	}
	return c.git, nil
}
