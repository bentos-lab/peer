package cli

import (
	"context"
	"errors"

	"github.com/bentos-lab/peer/shared/toolinstall"
)

// GhInstaller installs GitHub CLI and handles auth.
type GhInstaller interface {
	IsGhInstalled() bool
	IsGhAuthenticated(context.Context) (bool, error)
	EnsureGhInstalled(context.Context) error
	EnsureGhAuthenticated(context.Context) error
}

// GlabInstaller installs GitLab CLI and handles auth.
type GlabInstaller interface {
	IsGlabInstalled() bool
	IsGlabAuthenticated(context.Context) (bool, error)
	EnsureGlabInstalled(context.Context) error
	EnsureGlabAuthenticated(context.Context) error
}

// OpencodeInstaller installs OpenCode.
type OpencodeInstaller interface {
	IsOpencodeInstalled() bool
	EnsureOpencodeInstalled(context.Context) error
}

// GitInstaller installs Git.
type GitInstaller interface {
	IsGitInstalled() bool
	EnsureGitInstalled(context.Context) error
}

// InstallOutcome reports the install status for a tool.
type InstallOutcome struct {
	Installed            bool
	AlreadyAuthenticated bool
}

// QuickstartOutcome reports the install status for quickstart tools.
type QuickstartOutcome struct {
	Gh       InstallOutcome
	Opencode InstallOutcome
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
func (c *InstallCommand) InstallGh(ctx context.Context, login bool) (InstallOutcome, error) {
	installer, err := c.ghInstaller()
	if err != nil {
		return InstallOutcome{}, err
	}
	alreadyInstalled := installer.IsGhInstalled()
	if !alreadyInstalled {
		if err := installer.EnsureGhInstalled(ctx); err != nil {
			return InstallOutcome{}, err
		}
	}
	outcome := InstallOutcome{Installed: !alreadyInstalled}
	if !login {
		return outcome, nil
	}
	authenticated, err := installer.IsGhAuthenticated(ctx)
	if err != nil {
		return outcome, err
	}
	if authenticated {
		outcome.AlreadyAuthenticated = true
		return outcome, nil
	}
	if err := installer.EnsureGhAuthenticated(ctx); err != nil {
		return outcome, err
	}
	return outcome, nil
}

// InstallGlab installs GitLab CLI and optionally logs in.
func (c *InstallCommand) InstallGlab(ctx context.Context, login bool) (InstallOutcome, error) {
	installer, err := c.glabInstaller()
	if err != nil {
		return InstallOutcome{}, err
	}
	alreadyInstalled := installer.IsGlabInstalled()
	if !alreadyInstalled {
		if err := installer.EnsureGlabInstalled(ctx); err != nil {
			return InstallOutcome{}, err
		}
	}
	outcome := InstallOutcome{Installed: !alreadyInstalled}
	if !login {
		return outcome, nil
	}
	authenticated, err := installer.IsGlabAuthenticated(ctx)
	if err != nil {
		return outcome, err
	}
	if authenticated {
		outcome.AlreadyAuthenticated = true
		return outcome, nil
	}
	if err := installer.EnsureGlabAuthenticated(ctx); err != nil {
		return outcome, err
	}
	return outcome, nil
}

// InstallOpencode installs OpenCode (opencode).
func (c *InstallCommand) InstallOpencode(ctx context.Context) (InstallOutcome, error) {
	installer, err := c.opencodeInstaller()
	if err != nil {
		return InstallOutcome{}, err
	}
	alreadyInstalled := installer.IsOpencodeInstalled()
	if !alreadyInstalled {
		if err := installer.EnsureOpencodeInstalled(ctx); err != nil {
			return InstallOutcome{}, err
		}
	}
	return InstallOutcome{Installed: !alreadyInstalled}, nil
}

// InstallGit installs Git.
func (c *InstallCommand) InstallGit(ctx context.Context) (InstallOutcome, error) {
	installer, err := c.gitInstaller()
	if err != nil {
		return InstallOutcome{}, err
	}
	alreadyInstalled := installer.IsGitInstalled()
	if !alreadyInstalled {
		if err := installer.EnsureGitInstalled(ctx); err != nil {
			return InstallOutcome{}, err
		}
	}
	return InstallOutcome{Installed: !alreadyInstalled}, nil
}

// InstallQuickstart installs gh (with login) and opencode.
func (c *InstallCommand) InstallQuickstart(ctx context.Context) (QuickstartOutcome, error) {
	ghOutcome, err := c.InstallGh(ctx, true)
	if err != nil {
		return QuickstartOutcome{}, err
	}
	opencodeOutcome, err := c.InstallOpencode(ctx)
	if err != nil {
		return QuickstartOutcome{}, err
	}
	return QuickstartOutcome{Gh: ghOutcome, Opencode: opencodeOutcome}, nil
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
