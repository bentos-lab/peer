package toolinstall

import (
	"context"
	"errors"
	"fmt"
)

// GitInstaller installs git on the host.
type GitInstaller struct {
	base baseInstaller
}

// NewGitInstaller creates a git installer with optional deps.
func NewGitInstaller(deps *Deps) *GitInstaller {
	return &GitInstaller{base: newBaseInstaller(deps)}
}

// EnsureGitInstalled installs git when missing.
func (i *GitInstaller) EnsureGitInstalled(ctx context.Context) error {
	if i.base.commandAvailable("git") {
		return nil
	}
	if !i.base.isTerminal() {
		i.base.printGitInstructions()
		i.base.printManualActionHint("git", "install")
		return errors.New("git installation requires an interactive terminal")
	}
	ok, err := i.base.promptYesNo("Git not found. Install now? [Y/n]: ")
	if err != nil {
		return err
	}
	if !ok {
		i.base.printGitInstructions()
		i.base.printManualActionHint("git", "install")
		return errors.New("git not installed")
	}

	switch i.base.goos {
	case "darwin":
		if i.base.commandAvailable("brew") {
			if err := i.base.run(ctx, "brew", "install", "git"); err != nil {
				i.base.printManualActionHint("git", "install")
				return err
			}
			return nil
		}
		i.base.printGitInstructions()
		i.base.printManualActionHint("git", "install")
		return errors.New("homebrew not found for git install")
	case "linux":
		if err := i.installGitLinux(ctx); err != nil {
			i.base.printManualActionHint("git", "install")
			return err
		}
		return nil
	case "windows":
		if i.base.commandAvailable("winget") {
			if err := i.base.run(ctx, "winget", "install", "--id", "Git.Git"); err != nil {
				i.base.printManualActionHint("git", "install")
				return err
			}
			return nil
		}
		i.base.printGitInstructions()
		i.base.printManualActionHint("git", "install")
		return errors.New("winget not found for git install")
	default:
		i.base.printGitInstructions()
		i.base.printManualActionHint("git", "install")
		return fmt.Errorf("unsupported platform %q for git install", i.base.goos)
	}
}

func (i *GitInstaller) installGitLinux(ctx context.Context) error {
	switch {
	case i.base.commandAvailable("apt-get"):
		return i.base.runShell(ctx, gitAptInstallCommand)
	case i.base.commandAvailable("dnf"):
		return i.base.runShell(ctx, gitDnfInstallCommand)
	case i.base.commandAvailable("yum"):
		return i.base.runShell(ctx, gitYumInstallCommand)
	default:
		i.base.printGitInstructions()
		return errors.New("no supported package manager found for git install")
	}
}
