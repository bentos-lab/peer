package toolinstall

import (
	"context"
	"errors"
	"fmt"
)

// GlabInstaller installs the GitLab CLI.
type GlabInstaller struct {
	base            baseInstaller
	authHintPrinted bool
}

// NewGlabInstaller creates a GitLab CLI installer with optional deps.
func NewGlabInstaller(deps *Deps) *GlabInstaller {
	return &GlabInstaller{base: newBaseInstaller(deps)}
}

// IsGlabInstalled reports whether glab is available on PATH.
func (i *GlabInstaller) IsGlabInstalled() bool {
	return i.base.commandAvailable("glab")
}

// IsGlabAuthenticated reports whether glab auth is configured.
func (i *GlabInstaller) IsGlabAuthenticated(ctx context.Context) (bool, error) {
	if !i.base.commandAvailable("glab") {
		return false, errors.New("glab is not installed")
	}
	if err := i.base.run(ctx, "glab", "auth", "status"); err == nil {
		return true, nil
	}
	i.printAuthStatusHintOnce()
	return false, nil
}

// EnsureGlabInstalled installs glab when missing.
func (i *GlabInstaller) EnsureGlabInstalled(ctx context.Context) error {
	if i.base.commandAvailable("glab") {
		return nil
	}
	if !i.base.isTerminal() {
		i.base.printGlabInstructions()
		i.base.printManualActionHint("glab", "install")
		return errors.New("glab installation requires an interactive terminal")
	}
	ok, err := i.base.promptYesNo("GitLab CLI (glab) not found. Install now? [Y/n]: ")
	if err != nil {
		return err
	}
	if !ok {
		i.base.printGlabInstructions()
		i.base.printManualActionHint("glab", "install")
		return errors.New("glab not installed")
	}

	switch i.base.goos {
	case "darwin":
		if i.base.commandAvailable("brew") {
			if err := i.base.run(ctx, "brew", "install", "glab"); err != nil {
				i.base.printManualActionHint("glab", "install")
				return err
			}
			return nil
		}
		i.base.printGlabInstructions()
		i.base.printManualActionHint("glab", "install")
		return errors.New("homebrew not found for glab install")
	case "linux":
		if err := i.installGlabLinux(ctx); err != nil {
			i.base.printManualActionHint("glab", "install")
			return err
		}
		return nil
	case "windows":
		if i.base.commandAvailable("winget") {
			if err := i.base.run(ctx, "winget", "install", "glab.glab"); err != nil {
				i.base.printManualActionHint("glab", "install")
				return err
			}
			return nil
		}
		if i.base.commandAvailable("choco") {
			if err := i.base.run(ctx, "choco", "install", "glab"); err != nil {
				i.base.printManualActionHint("glab", "install")
				return err
			}
			return nil
		}
		if i.base.commandAvailable("scoop") {
			if err := i.base.run(ctx, "scoop", "install", "glab"); err != nil {
				i.base.printManualActionHint("glab", "install")
				return err
			}
			return nil
		}
		i.base.printGlabInstructions()
		i.base.printManualActionHint("glab", "install")
		return errors.New("no supported package manager found for glab install")
	default:
		i.base.printGlabInstructions()
		i.base.printManualActionHint("glab", "install")
		return fmt.Errorf("unsupported platform %q for glab install", i.base.goos)
	}
}

// EnsureGlabAuthenticated checks glab auth and optionally prompts to login.
func (i *GlabInstaller) EnsureGlabAuthenticated(ctx context.Context) error {
	if !i.base.commandAvailable("glab") {
		return errors.New("glab is not installed")
	}
	if err := i.base.run(ctx, "glab", "auth", "status"); err == nil {
		return nil
	}
	i.printAuthStatusHintOnce()
	if !i.base.isTerminal() {
		_, _ = fmt.Fprintln(i.base.stderr, "glab is not authenticated. Skipping 'glab auth login' because no TTY is available.")
		return nil
	}
	ok, err := i.base.promptYesNo("glab is not authenticated. Run 'glab auth login' now? [Y/n]: ")
	if err != nil {
		return err
	}
	if !ok {
		_, _ = fmt.Fprintln(i.base.stderr, "glab is not authenticated. Run: glab auth login")
		return errors.New("glab is not authenticated")
	}
	if err := i.base.run(ctx, "glab", "auth", "login"); err != nil {
		i.base.printManualActionHint("glab", "auth login")
		return err
	}
	return nil
}

func (i *GlabInstaller) printAuthStatusHintOnce() {
	if i.authHintPrinted {
		return
	}
	i.authHintPrinted = true
	_, _ = fmt.Fprintln(i.base.stderr, "Note: glab auth status reads credential files; in sandboxed runs you may need to grant read permission to those files.")
}

func (i *GlabInstaller) installGlabLinux(ctx context.Context) error {
	switch {
	case i.base.commandAvailable("apt"):
		return i.base.runShell(ctx, glabAptInstallCommand)
	case i.base.commandAvailable("dnf"):
		return i.base.runShell(ctx, glabDnfInstallCommand)
	default:
		i.base.printGlabInstructions()
		return errors.New("no supported package manager found for glab install")
	}
}
