package toolinstall

import (
	"context"
	"errors"
	"fmt"
)

// GhInstaller installs the GitHub CLI.
type GhInstaller struct {
	base            baseInstaller
	authHintPrinted bool
}

// NewGhInstaller creates a GitHub CLI installer with optional deps.
func NewGhInstaller(deps *Deps) *GhInstaller {
	return &GhInstaller{base: newBaseInstaller(deps)}
}

// IsGhInstalled reports whether gh is available on PATH.
func (i *GhInstaller) IsGhInstalled() bool {
	return i.base.commandAvailable("gh")
}

// IsGhAuthenticated reports whether gh auth is configured.
func (i *GhInstaller) IsGhAuthenticated(ctx context.Context) (bool, error) {
	if !i.base.commandAvailable("gh") {
		return false, errors.New("gh is not installed")
	}
	if err := i.base.run(ctx, "gh", "auth", "status"); err == nil {
		return true, nil
	}
	i.printAuthStatusHintOnce()
	return false, nil
}

// EnsureGhInstalled installs gh when missing.
func (i *GhInstaller) EnsureGhInstalled(ctx context.Context) error {
	if i.base.commandAvailable("gh") {
		return nil
	}
	if !i.base.isTerminal() {
		i.base.printGhInstructions()
		i.base.printManualActionHint("gh", "install")
		return errors.New("gh installation requires an interactive terminal")
	}
	ok, err := i.base.promptYesNo("GitHub CLI (gh) not found. Install now? [Y/n]: ")
	if err != nil {
		return err
	}
	if !ok {
		i.base.printGhInstructions()
		i.base.printManualActionHint("gh", "install")
		return errors.New("gh not installed")
	}

	switch i.base.goos {
	case "darwin":
		if i.base.commandAvailable("brew") {
			if err := i.base.run(ctx, "brew", "install", "gh"); err != nil {
				i.base.printManualActionHint("gh", "install")
				return err
			}
			return nil
		}
		i.base.printGhInstructions()
		i.base.printManualActionHint("gh", "install")
		return errors.New("homebrew not found for gh install")
	case "linux":
		if err := i.installGhLinux(ctx); err != nil {
			i.base.printManualActionHint("gh", "install")
			return err
		}
		return nil
	case "windows":
		if i.base.commandAvailable("winget") {
			if err := i.base.run(ctx, "winget", "install", "--id", "GitHub.cli"); err != nil {
				i.base.printManualActionHint("gh", "install")
				return err
			}
			return nil
		}
		i.base.printGhInstructions()
		i.base.printManualActionHint("gh", "install")
		return errors.New("winget not found for gh install")
	default:
		i.base.printGhInstructions()
		i.base.printManualActionHint("gh", "install")
		return fmt.Errorf("unsupported platform %q for gh install", i.base.goos)
	}
}

// EnsureGhAuthenticated checks gh auth and optionally prompts to login.
func (i *GhInstaller) EnsureGhAuthenticated(ctx context.Context) error {
	if !i.base.commandAvailable("gh") {
		return errors.New("gh is not installed")
	}
	if err := i.base.run(ctx, "gh", "auth", "status"); err == nil {
		return nil
	}
	i.printAuthStatusHintOnce()
	if !i.base.isTerminal() {
		_, _ = fmt.Fprintln(i.base.stderr, "gh is not authenticated. Skipping 'gh auth login' because no TTY is available.")
		return nil
	}
	ok, err := i.base.promptYesNo("gh is not authenticated. Run 'gh auth login' now? [Y/n]: ")
	if err != nil {
		return err
	}
	if !ok {
		_, _ = fmt.Fprintln(i.base.stderr, "gh is not authenticated. Run: gh auth login")
		return errors.New("gh is not authenticated")
	}
	if err := i.base.run(ctx, "gh", "auth", "login"); err != nil {
		i.base.printManualActionHint("gh", "auth login")
		return err
	}
	return nil
}

func (i *GhInstaller) printAuthStatusHintOnce() {
	if i.authHintPrinted {
		return
	}
	i.authHintPrinted = true
	_, _ = fmt.Fprintln(i.base.stderr, "Note: gh auth status reads credential files; in sandboxed runs you may need to grant read permission to those files.")
}

func (i *GhInstaller) installGhLinux(ctx context.Context) error {
	switch {
	case i.base.commandAvailable("apt-get"):
		return i.base.runShell(ctx, ghAptInstallCommand)
	case i.base.commandAvailable("dnf5"):
		return i.base.runShell(ctx, ghDnf5InstallCommand)
	case i.base.commandAvailable("dnf"):
		return i.base.runShell(ctx, ghDnf4InstallCommand)
	case i.base.commandAvailable("yum"):
		return i.base.runShell(ctx, ghYumInstallCommand)
	default:
		i.base.printGhInstructions()
		return errors.New("no supported package manager found for gh install")
	}
}
