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

// Deps provides optional overrides for tool installers.
type Deps struct {
	StreamRunner commandrunner.StreamRunner
	LookPath     func(string) (string, error)
	Stdin        io.Reader
	Stdout       io.Writer
	Stderr       io.Writer
	GOOS         string
	IsTerminal   func() bool
	PreferTTY    *bool
}

type baseInstaller struct {
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

func newBaseInstaller(deps *Deps) baseInstaller {
	if deps == nil {
		deps = &Deps{}
	}
	stdin := deps.Stdin
	if stdin == nil {
		stdin = os.Stdin
	}
	stdout := deps.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	stderr := deps.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}
	lookPath := deps.LookPath
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	streamRunner := deps.StreamRunner
	if streamRunner == nil {
		streamRunner = commandrunner.NewOSStreamCommandRunner()
	}
	ttyRunner := commandrunner.NewOSTTYCommandRunner()
	preferTTY := true
	if deps.PreferTTY != nil {
		preferTTY = *deps.PreferTTY
	}
	goos := deps.GOOS
	if goos == "" {
		goos = runtime.GOOS
	}
	isTerminal := deps.IsTerminal
	if isTerminal == nil {
		isTerminal = func() bool { return isTerminalReader(stdin) }
	}

	return baseInstaller{
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

func (i *baseInstaller) commandAvailable(name string) bool {
	_, err := i.lookPath(name)
	return err == nil
}

func (i *baseInstaller) run(ctx context.Context, name string, args ...string) error {
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

func (i *baseInstaller) runShell(ctx context.Context, script string) error {
	return i.run(ctx, "bash", "-c", script)
}

func (i *baseInstaller) runWithStreaming(ctx context.Context, name string, args ...string) (commandrunner.Result, error) {
	if i.preferTTY && i.isTerminal() {
		return commandrunner.Result{}, i.ttyRunner.RunTTY(ctx, name, args...)
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

func (i *baseInstaller) promptYesNo(prompt string) (bool, error) {
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

func (i *baseInstaller) printOpencodeInstructions() {
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

func (i *baseInstaller) printGhInstructions() {
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

func (i *baseInstaller) printGlabInstructions() {
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

func (i *baseInstaller) printGitInstructions() {
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

func (i *baseInstaller) printManualActionHint(tool string, action string) {
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
