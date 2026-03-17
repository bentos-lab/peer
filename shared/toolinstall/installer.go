package toolinstall

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"bentos-backend/adapter/outbound/commandrunner"
)

const (
	opencodeInstallScript = "curl -fsSL https://opencode.ai/install | bash"
	ghInstallHint         = "GitHub CLI can be installed from official releases if package managers are unavailable."
	glabInstallHint       = "GitLab CLI can be installed from official releases if package managers are unavailable."
	gitInstallHint        = "Git can be installed from official releases if package managers are unavailable."
)

var (
	ghAptInstallCommand = strings.Join([]string{
		"(type -p wget >/dev/null || (sudo apt update && sudo apt install wget -y))",
		"sudo mkdir -p -m 755 /etc/apt/keyrings",
		"out=$(mktemp) && wget -nv -O$out https://cli.github.com/packages/githubcli-archive-keyring.gpg",
		"cat $out | sudo tee /etc/apt/keyrings/githubcli-archive-keyring.gpg > /dev/null",
		"sudo chmod go+r /etc/apt/keyrings/githubcli-archive-keyring.gpg",
		"sudo mkdir -p -m 755 /etc/apt/sources.list.d",
		"echo \"deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main\" | sudo tee /etc/apt/sources.list.d/github-cli.list > /dev/null",
		"sudo apt update",
		"sudo apt install gh -y",
	}, " && ")
	ghDnf5InstallCommand = strings.Join([]string{
		"sudo dnf install dnf5-plugins",
		"sudo dnf config-manager addrepo --from-repofile=https://cli.github.com/packages/rpm/gh-cli.repo",
		"sudo dnf install gh --repo gh-cli",
	}, " && ")
	ghDnf4InstallCommand = strings.Join([]string{
		"sudo dnf install 'dnf-command(config-manager)'",
		"sudo dnf config-manager --add-repo https://cli.github.com/packages/rpm/gh-cli.repo",
		"sudo dnf install gh --repo gh-cli",
	}, " && ")
	ghYumInstallCommand = strings.Join([]string{
		"type -p yum-config-manager >/dev/null || sudo yum install yum-utils",
		"sudo yum-config-manager --add-repo https://cli.github.com/packages/rpm/gh-cli.repo",
		"sudo yum install gh",
	}, " && ")

	glabAptInstallCommand = strings.Join([]string{
		"curl -sSL https://raw.githubusercontent.com/upciti/wakemeops/main/assets/install_repository | sudo bash",
		"sudo apt install glab",
	}, " && ")
	glabDnfInstallCommand = "sudo dnf install -y glab"

	gitAptInstallCommand = strings.Join([]string{
		"sudo apt-get update",
		"sudo apt-get install git -y",
	}, " && ")
	gitDnfInstallCommand = "sudo dnf install -y git"
	gitYumInstallCommand = "sudo yum install -y git"
)

// Config configures the installer dependencies.
type Config struct {
	StreamRunner commandrunner.StreamRunner
	TTYRunner    commandrunner.TTYRunner
	PreferTTY    bool
	PreferTTYSet bool
	LookPath     func(string) (string, error)
	Stdin        io.Reader
	Stdout       io.Writer
	Stderr       io.Writer
	GOOS         string
	IsTerminal   func() bool
}

// Installer installs required CLI tools on the host.
type Installer struct {
	streamRunner commandrunner.StreamRunner
	ttyRunner    commandrunner.TTYRunner
	preferTTY    bool
	lookPath     func(string) (string, error)
	stdin        io.Reader
	stdout       io.Writer
	stderr       io.Writer
	goos         string
	isTerminal   func() bool
}

// NewInstaller creates a new Installer with defaults.
func NewInstaller(cfg Config) *Installer {
	stdin := cfg.Stdin
	if stdin == nil {
		stdin = os.Stdin
	}
	stdout := cfg.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	stderr := cfg.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}
	lookPath := cfg.LookPath
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	streamRunner := cfg.StreamRunner
	if streamRunner == nil {
		streamRunner = commandrunner.NewOSStreamCommandRunner()
	}
	ttyRunner := cfg.TTYRunner
	if ttyRunner == nil {
		ttyRunner = commandrunner.NewOSTTYCommandRunner()
	}
	preferTTY := true
	if cfg.PreferTTYSet {
		preferTTY = cfg.PreferTTY
	}
	goos := cfg.GOOS
	if goos == "" {
		goos = runtime.GOOS
	}
	isTerminal := cfg.IsTerminal
	if isTerminal == nil {
		isTerminal = func() bool { return isTerminalReader(stdin) }
	}

	return &Installer{
		streamRunner: streamRunner,
		ttyRunner:    ttyRunner,
		preferTTY:    preferTTY,
		lookPath:     lookPath,
		stdin:        stdin,
		stdout:       stdout,
		stderr:       stderr,
		goos:         goos,
		isTerminal:   isTerminal,
	}
}

// EnsureOpencodeInstalled installs opencode when missing.
func (i *Installer) EnsureOpencodeInstalled(ctx context.Context) error {
	if i.commandAvailable("opencode") {
		return nil
	}
	if !i.isTerminal() {
		i.printOpencodeInstructions()
		return errors.New("opencode installation requires an interactive terminal")
	}
	ok, err := i.promptYesNo("opencode not found. Install now? [Y/n]: ")
	if err != nil {
		return err
	}
	if !ok {
		i.printOpencodeInstructions()
		return errors.New("opencode not installed")
	}

	switch i.goos {
	case "darwin", "linux":
		if !i.commandAvailable("curl") {
			i.printOpencodeInstructions()
			return errors.New("curl is required to install opencode")
		}
		if !i.commandAvailable("bash") {
			i.printOpencodeInstructions()
			return errors.New("bash is required to install opencode")
		}
		return i.runShell(ctx, opencodeInstallScript)
	case "windows":
		return i.installOpencodeWindows(ctx)
	default:
		i.printOpencodeInstructions()
		return fmt.Errorf("unsupported platform %q for opencode install", i.goos)
	}
}

// EnsureGhInstalled installs gh when missing.
func (i *Installer) EnsureGhInstalled(ctx context.Context) error {
	if i.commandAvailable("gh") {
		return nil
	}
	if !i.isTerminal() {
		i.printGhInstructions()
		i.printManualActionHint("gh", "install")
		return errors.New("gh installation requires an interactive terminal")
	}
	ok, err := i.promptYesNo("GitHub CLI (gh) not found. Install now? [Y/n]: ")
	if err != nil {
		return err
	}
	if !ok {
		i.printGhInstructions()
		i.printManualActionHint("gh", "install")
		return errors.New("gh not installed")
	}

	switch i.goos {
	case "darwin":
		if i.commandAvailable("brew") {
			if err := i.run(ctx, "brew", "install", "gh"); err != nil {
				i.printManualActionHint("gh", "install")
				return err
			}
			return nil
		}
		i.printGhInstructions()
		i.printManualActionHint("gh", "install")
		return errors.New("homebrew not found for gh install")
	case "linux":
		if err := i.installGhLinux(ctx); err != nil {
			i.printManualActionHint("gh", "install")
			return err
		}
		return nil
	case "windows":
		if i.commandAvailable("winget") {
			if err := i.run(ctx, "winget", "install", "--id", "GitHub.cli"); err != nil {
				i.printManualActionHint("gh", "install")
				return err
			}
			return nil
		}
		i.printGhInstructions()
		i.printManualActionHint("gh", "install")
		return errors.New("winget not found for gh install")
	default:
		i.printGhInstructions()
		i.printManualActionHint("gh", "install")
		return fmt.Errorf("unsupported platform %q for gh install", i.goos)
	}
}

// EnsureGhAuthenticated checks gh auth and optionally prompts to login.
func (i *Installer) EnsureGhAuthenticated(ctx context.Context) error {
	if !i.commandAvailable("gh") {
		return errors.New("gh is not installed")
	}
	if err := i.run(ctx, "gh", "auth", "status"); err == nil {
		return nil
	}
	if !i.isTerminal() {
		_, _ = fmt.Fprintln(i.stderr, "gh is not authenticated. Skipping 'gh auth login' because no TTY is available.")
		return nil
	}
	ok, err := i.promptYesNo("gh is not authenticated. Run 'gh auth login' now? [Y/n]: ")
	if err != nil {
		return err
	}
	if !ok {
		_, _ = fmt.Fprintln(i.stderr, "gh is not authenticated. Run: gh auth login")
		return errors.New("gh is not authenticated")
	}
	if err := i.run(ctx, "gh", "auth", "login"); err != nil {
		i.printManualActionHint("gh", "auth login")
		return err
	}
	return nil
}

// EnsureGlabInstalled installs glab when missing.
func (i *Installer) EnsureGlabInstalled(ctx context.Context) error {
	if i.commandAvailable("glab") {
		return nil
	}
	if !i.isTerminal() {
		i.printGlabInstructions()
		i.printManualActionHint("glab", "install")
		return errors.New("glab installation requires an interactive terminal")
	}
	ok, err := i.promptYesNo("GitLab CLI (glab) not found. Install now? [Y/n]: ")
	if err != nil {
		return err
	}
	if !ok {
		i.printGlabInstructions()
		i.printManualActionHint("glab", "install")
		return errors.New("glab not installed")
	}

	switch i.goos {
	case "darwin":
		if i.commandAvailable("brew") {
			if err := i.run(ctx, "brew", "install", "glab"); err != nil {
				i.printManualActionHint("glab", "install")
				return err
			}
			return nil
		}
		i.printGlabInstructions()
		i.printManualActionHint("glab", "install")
		return errors.New("homebrew not found for glab install")
	case "linux":
		if err := i.installGlabLinux(ctx); err != nil {
			i.printManualActionHint("glab", "install")
			return err
		}
		return nil
	case "windows":
		if i.commandAvailable("winget") {
			if err := i.run(ctx, "winget", "install", "glab.glab"); err != nil {
				i.printManualActionHint("glab", "install")
				return err
			}
			return nil
		}
		if i.commandAvailable("choco") {
			if err := i.run(ctx, "choco", "install", "glab"); err != nil {
				i.printManualActionHint("glab", "install")
				return err
			}
			return nil
		}
		if i.commandAvailable("scoop") {
			if err := i.run(ctx, "scoop", "install", "glab"); err != nil {
				i.printManualActionHint("glab", "install")
				return err
			}
			return nil
		}
		i.printGlabInstructions()
		i.printManualActionHint("glab", "install")
		return errors.New("no supported package manager found for glab install")
	default:
		i.printGlabInstructions()
		i.printManualActionHint("glab", "install")
		return fmt.Errorf("unsupported platform %q for glab install", i.goos)
	}
}

// EnsureGitInstalled installs git when missing.
func (i *Installer) EnsureGitInstalled(ctx context.Context) error {
	if i.commandAvailable("git") {
		return nil
	}
	if !i.isTerminal() {
		i.printGitInstructions()
		i.printManualActionHint("git", "install")
		return errors.New("git installation requires an interactive terminal")
	}
	ok, err := i.promptYesNo("Git not found. Install now? [Y/n]: ")
	if err != nil {
		return err
	}
	if !ok {
		i.printGitInstructions()
		i.printManualActionHint("git", "install")
		return errors.New("git not installed")
	}

	switch i.goos {
	case "darwin":
		if i.commandAvailable("brew") {
			if err := i.run(ctx, "brew", "install", "git"); err != nil {
				i.printManualActionHint("git", "install")
				return err
			}
			return nil
		}
		i.printGitInstructions()
		i.printManualActionHint("git", "install")
		return errors.New("homebrew not found for git install")
	case "linux":
		if err := i.installGitLinux(ctx); err != nil {
			i.printManualActionHint("git", "install")
			return err
		}
		return nil
	case "windows":
		if i.commandAvailable("winget") {
			if err := i.run(ctx, "winget", "install", "--id", "Git.Git"); err != nil {
				i.printManualActionHint("git", "install")
				return err
			}
			return nil
		}
		i.printGitInstructions()
		i.printManualActionHint("git", "install")
		return errors.New("winget not found for git install")
	default:
		i.printGitInstructions()
		i.printManualActionHint("git", "install")
		return fmt.Errorf("unsupported platform %q for git install", i.goos)
	}
}

// EnsureGlabAuthenticated checks glab auth and optionally prompts to login.
func (i *Installer) EnsureGlabAuthenticated(ctx context.Context) error {
	if !i.commandAvailable("glab") {
		return errors.New("glab is not installed")
	}
	if err := i.run(ctx, "glab", "auth", "status"); err == nil {
		return nil
	}
	if !i.isTerminal() {
		_, _ = fmt.Fprintln(i.stderr, "glab is not authenticated. Skipping 'glab auth login' because no TTY is available.")
		return nil
	}
	ok, err := i.promptYesNo("glab is not authenticated. Run 'glab auth login' now? [Y/n]: ")
	if err != nil {
		return err
	}
	if !ok {
		_, _ = fmt.Fprintln(i.stderr, "glab is not authenticated. Run: glab auth login")
		return errors.New("glab is not authenticated")
	}
	if err := i.run(ctx, "glab", "auth", "login"); err != nil {
		i.printManualActionHint("glab", "auth login")
		return err
	}
	return nil
}

func (i *Installer) installOpencodeWindows(ctx context.Context) error {
	switch {
	case i.commandAvailable("choco"):
		return i.run(ctx, "choco", "install", "opencode")
	case i.commandAvailable("scoop"):
		return i.run(ctx, "scoop", "install", "opencode")
	case i.commandAvailable("npm"):
		return i.run(ctx, "npm", "install", "-g", "opencode-ai")
	default:
		i.printOpencodeInstructions()
		return errors.New("opencode is not installed")
	}
}

func (i *Installer) installGhLinux(ctx context.Context) error {
	switch {
	case i.commandAvailable("apt-get"):
		return i.runShell(ctx, ghAptInstallCommand)
	case i.commandAvailable("dnf5"):
		return i.runShell(ctx, ghDnf5InstallCommand)
	case i.commandAvailable("dnf"):
		return i.runShell(ctx, ghDnf4InstallCommand)
	case i.commandAvailable("yum"):
		return i.runShell(ctx, ghYumInstallCommand)
	default:
		i.printGhInstructions()
		return errors.New("no supported package manager found for gh install")
	}
}

func (i *Installer) installGlabLinux(ctx context.Context) error {
	switch {
	case i.commandAvailable("apt"):
		return i.runShell(ctx, glabAptInstallCommand)
	case i.commandAvailable("dnf"):
		return i.runShell(ctx, glabDnfInstallCommand)
	default:
		i.printGlabInstructions()
		return errors.New("no supported package manager found for glab install")
	}
}

func (i *Installer) installGitLinux(ctx context.Context) error {
	switch {
	case i.commandAvailable("apt-get"):
		return i.runShell(ctx, gitAptInstallCommand)
	case i.commandAvailable("dnf"):
		return i.runShell(ctx, gitDnfInstallCommand)
	case i.commandAvailable("yum"):
		return i.runShell(ctx, gitYumInstallCommand)
	default:
		i.printGitInstructions()
		return errors.New("no supported package manager found for git install")
	}
}

func (i *Installer) commandAvailable(name string) bool {
	_, err := i.lookPath(name)
	return err == nil
}

func (i *Installer) run(ctx context.Context, name string, args ...string) error {
	result, err := i.runWithStreaming(ctx, name, args...)
	if err == nil {
		return nil
	}
	message := strings.TrimSpace(string(result.Stderr))
	if message == "" {
		message = strings.TrimSpace(string(result.Stdout))
	}
	if message == "" {
		return err
	}
	return fmt.Errorf("%w: %s", err, message)
}

func (i *Installer) runShell(ctx context.Context, script string) error {
	return i.run(ctx, "bash", "-c", script)
}

func (i *Installer) runWithStreaming(ctx context.Context, name string, args ...string) (commandrunner.Result, error) {
	if i.preferTTY && i.isTerminal() {
		if i.ttyRunner == nil {
			return commandrunner.Result{}, errors.New("tty runner not configured")
		}
		return commandrunner.Result{}, i.ttyRunner.RunTTY(ctx, name, args...)
	}
	if i.streamRunner == nil {
		return commandrunner.Result{}, errors.New("stream runner not configured")
	}
	return i.streamRunner.RunStream(ctx, func(chunk commandrunner.StreamChunk) {
		switch chunk.Type {
		case commandrunner.StreamTypeStdout:
			if i.stdout != nil {
				_, _ = i.stdout.Write(chunk.Data)
			}
		case commandrunner.StreamTypeStderr:
			if i.stderr != nil {
				_, _ = i.stderr.Write(chunk.Data)
			}
		}
	}, name, args...)
}

func (i *Installer) promptYesNo(prompt string) (bool, error) {
	if !i.isTerminal() {
		return false, errors.New("interactive terminal required")
	}
	_, _ = fmt.Fprint(i.stdout, prompt)
	reader := bufio.NewReader(i.stdin)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	line = strings.TrimSpace(line)
	switch strings.ToLower(line) {
	case "":
		return true, nil
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}

func (i *Installer) printOpencodeInstructions() {
	_, _ = fmt.Fprintln(i.stderr, "OpenCode install options:")
	_, _ = fmt.Fprintf(i.stderr, "- Install script (macOS/Linux): %s\n", opencodeInstallScript)
	_, _ = fmt.Fprintln(i.stderr, "- npm: npm install -g opencode-ai")
	_, _ = fmt.Fprintln(i.stderr, "- Homebrew: brew install anomalyco/tap/opencode")
	_, _ = fmt.Fprintln(i.stderr, "- Windows (Chocolatey): choco install opencode")
	_, _ = fmt.Fprintln(i.stderr, "- Windows (Scoop): scoop install opencode")
	_, _ = fmt.Fprintln(i.stderr, "- Windows (NPM): npm install -g opencode-ai")
	_, _ = fmt.Fprintln(i.stderr, "- Windows (Mise): mise use -g github:anomalyco/opencode")
	_, _ = fmt.Fprintln(i.stderr, "- Windows (Docker): docker run -it --rm ghcr.io/anomalyco/opencode")
	_, _ = fmt.Fprintln(i.stderr, "- Releases: https://opencode.ai/docs")
}

func (i *Installer) printGhInstructions() {
	_, _ = fmt.Fprintln(i.stderr, "GitHub CLI install options:")
	_, _ = fmt.Fprintln(i.stderr, "- macOS Homebrew: brew install gh")
	_, _ = fmt.Fprintln(i.stderr, "- Linux (Debian/Ubuntu):")
	_, _ = fmt.Fprintf(i.stderr, "  %s\n", ghAptInstallCommand)
	_, _ = fmt.Fprintln(i.stderr, "- Linux (DNF5):")
	_, _ = fmt.Fprintf(i.stderr, "  %s\n", ghDnf5InstallCommand)
	_, _ = fmt.Fprintln(i.stderr, "- Linux (DNF4):")
	_, _ = fmt.Fprintf(i.stderr, "  %s\n", ghDnf4InstallCommand)
	_, _ = fmt.Fprintln(i.stderr, "- Linux (Yum):")
	_, _ = fmt.Fprintf(i.stderr, "  %s\n", ghYumInstallCommand)
	_, _ = fmt.Fprintln(i.stderr, "- Windows (WinGet): winget install --id GitHub.cli")
	_, _ = fmt.Fprintf(i.stderr, "- %s\n", ghInstallHint)
	_, _ = fmt.Fprintln(i.stderr, "- Releases: https://github.com/cli/cli/releases")
}

func (i *Installer) printGlabInstructions() {
	_, _ = fmt.Fprintln(i.stderr, "GitLab CLI install options:")
	_, _ = fmt.Fprintln(i.stderr, "- macOS Homebrew: brew install glab")
	_, _ = fmt.Fprintln(i.stderr, "- Linux (Debian/Ubuntu):")
	_, _ = fmt.Fprintf(i.stderr, "  %s\n", glabAptInstallCommand)
	_, _ = fmt.Fprintln(i.stderr, "- Linux (DNF):")
	_, _ = fmt.Fprintf(i.stderr, "  %s\n", glabDnfInstallCommand)
	_, _ = fmt.Fprintln(i.stderr, "- Windows (WinGet): winget install glab.glab")
	_, _ = fmt.Fprintln(i.stderr, "- Windows (Chocolatey): choco install glab")
	_, _ = fmt.Fprintln(i.stderr, "- Windows (Scoop): scoop install glab")
	_, _ = fmt.Fprintf(i.stderr, "- %s\n", glabInstallHint)
	_, _ = fmt.Fprintln(i.stderr, "- Releases: https://gitlab.com/gitlab-org/cli/-/releases")
}

func (i *Installer) printGitInstructions() {
	_, _ = fmt.Fprintln(i.stderr, "Git install options:")
	_, _ = fmt.Fprintln(i.stderr, "- macOS Homebrew: brew install git")
	_, _ = fmt.Fprintln(i.stderr, "- Linux (Debian/Ubuntu):")
	_, _ = fmt.Fprintf(i.stderr, "  %s\n", gitAptInstallCommand)
	_, _ = fmt.Fprintln(i.stderr, "- Linux (DNF):")
	_, _ = fmt.Fprintf(i.stderr, "  %s\n", gitDnfInstallCommand)
	_, _ = fmt.Fprintln(i.stderr, "- Linux (Yum):")
	_, _ = fmt.Fprintf(i.stderr, "  %s\n", gitYumInstallCommand)
	_, _ = fmt.Fprintln(i.stderr, "- Windows (WinGet): winget install --id Git.Git")
	_, _ = fmt.Fprintf(i.stderr, "- %s\n", gitInstallHint)
	_, _ = fmt.Fprintln(i.stderr, "- Releases: https://git-scm.com/downloads")
}

func (i *Installer) printManualActionHint(tool string, action string) {
	tool = strings.TrimSpace(tool)
	action = strings.TrimSpace(action)
	if tool == "" || action == "" {
		return
	}
	_, _ = fmt.Fprintf(i.stderr, "Manual action required: run `%s %s`.\n", tool, action)
}

func isTerminalReader(reader io.Reader) bool {
	file, ok := reader.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}
