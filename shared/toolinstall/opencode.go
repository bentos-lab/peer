package toolinstall

import (
	"context"
	"errors"
	"fmt"
)

// OpencodeInstaller installs opencode on the host.
type OpencodeInstaller struct {
	base baseInstaller
}

// NewOpencodeInstaller creates an opencode installer with optional deps.
func NewOpencodeInstaller(deps *Deps) *OpencodeInstaller {
	return &OpencodeInstaller{base: newBaseInstaller(deps)}
}

// IsOpencodeInstalled reports whether opencode is available on PATH.
func (i *OpencodeInstaller) IsOpencodeInstalled() bool {
	return i.base.commandAvailable("opencode")
}

// EnsureOpencodeInstalled installs opencode when missing.
func (i *OpencodeInstaller) EnsureOpencodeInstalled(ctx context.Context) error {
	if i.base.commandAvailable("opencode") {
		return nil
	}
	if !i.base.isTerminal() {
		i.base.printOpencodeInstructions()
		return errors.New("opencode installation requires an interactive terminal")
	}
	ok, err := i.base.promptYesNo("opencode not found. Install now? [Y/n]: ")
	if err != nil {
		return err
	}
	if !ok {
		i.base.printOpencodeInstructions()
		return errors.New("opencode not installed")
	}

	switch i.base.goos {
	case "darwin", "linux":
		if !i.base.commandAvailable("curl") {
			i.base.printOpencodeInstructions()
			return errors.New("curl is required to install opencode")
		}
		if !i.base.commandAvailable("bash") {
			i.base.printOpencodeInstructions()
			return errors.New("bash is required to install opencode")
		}
		return i.base.run(ctx, "bash", "-c", opencodeInstallScript)
	case "windows":
		return i.installOpencodeWindows(ctx)
	default:
		i.base.printOpencodeInstructions()
		return fmt.Errorf("unsupported platform %q for opencode install", i.base.goos)
	}
}

func (i *OpencodeInstaller) installOpencodeWindows(ctx context.Context) error {
	switch {
	case i.base.commandAvailable("choco"):
		return i.base.run(ctx, "choco", "install", "opencode")
	case i.base.commandAvailable("scoop"):
		return i.base.run(ctx, "scoop", "install", "opencode")
	case i.base.commandAvailable("npm"):
		return i.base.run(ctx, "npm", "install", "-g", "opencode-ai")
	default:
		i.base.printOpencodeInstructions()
		return errors.New("opencode is not installed")
	}
}
