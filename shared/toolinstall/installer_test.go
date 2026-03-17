package toolinstall

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"bentos-backend/adapter/outbound/commandrunner"

	"github.com/stretchr/testify/require"
)

func TestEnsureOpencodeInstalledRunsInstallScript(t *testing.T) {
	runner := commandrunner.NewDummyCommandRunner()
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{Name: "bash", Args: []string{"-c", opencodeInstallScript}},
		Result:   commandrunner.Result{},
	})

	stdin := bytes.NewBufferString("y\n")
	installer := NewInstaller(Config{
		StreamRunner: runner,
		PreferTTY:    false,
		PreferTTYSet: true,
		LookPath: func(name string) (string, error) {
			switch name {
			case "opencode":
				return "", errors.New("missing")
			case "curl", "bash":
				return "/bin/" + name, nil
			default:
				return "", errors.New("missing")
			}
		},
		Stdin:      stdin,
		Stdout:     &bytes.Buffer{},
		Stderr:     &bytes.Buffer{},
		GOOS:       "linux",
		IsTerminal: func() bool { return true },
	})

	err := installer.EnsureOpencodeInstalled(context.Background())
	require.NoError(t, err)
	require.NoError(t, runner.VerifyDone())
}

func TestEnsureOpencodeInstalledStreamsOutput(t *testing.T) {
	runner := commandrunner.NewDummyCommandRunner()
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{Name: "bash", Args: []string{"-c", opencodeInstallScript}},
		Result: commandrunner.Result{
			Stdout: []byte("install output\n"),
			Stderr: []byte("install warning\n"),
		},
	})

	stdin := bytes.NewBufferString("y\n")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	installer := NewInstaller(Config{
		StreamRunner: runner,
		PreferTTY:    false,
		PreferTTYSet: true,
		LookPath: func(name string) (string, error) {
			switch name {
			case "opencode":
				return "", errors.New("missing")
			case "curl", "bash":
				return "/bin/" + name, nil
			default:
				return "", errors.New("missing")
			}
		},
		Stdin:      stdin,
		Stdout:     stdout,
		Stderr:     stderr,
		GOOS:       "linux",
		IsTerminal: func() bool { return true },
	})

	err := installer.EnsureOpencodeInstalled(context.Background())
	require.NoError(t, err)
	require.NoError(t, runner.VerifyDone())
	require.Contains(t, stdout.String(), "install output")
	require.Contains(t, stderr.String(), "install warning")
}

func TestEnsureGhInstalledUsesAptWhenAvailable(t *testing.T) {
	runner := commandrunner.NewDummyCommandRunner()
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{Name: "bash", Args: []string{"-c", ghAptInstallCommand}},
		Result:   commandrunner.Result{},
	})

	stdin := bytes.NewBufferString("y\n")
	installer := NewInstaller(Config{
		StreamRunner: runner,
		PreferTTY:    false,
		PreferTTYSet: true,
		LookPath: func(name string) (string, error) {
			switch name {
			case "gh":
				return "", errors.New("missing")
			case "apt-get":
				return "/usr/bin/apt-get", nil
			default:
				return "", errors.New("missing")
			}
		},
		Stdin:      stdin,
		Stdout:     &bytes.Buffer{},
		Stderr:     &bytes.Buffer{},
		GOOS:       "linux",
		IsTerminal: func() bool { return true },
	})

	err := installer.EnsureGhInstalled(context.Background())
	require.NoError(t, err)
	require.NoError(t, runner.VerifyDone())
}

func TestEnsureGhInstalledNonInteractive(t *testing.T) {
	stderr := &bytes.Buffer{}
	installer := NewInstaller(Config{
		StreamRunner: commandrunner.NewDummyCommandRunner(),
		LookPath: func(name string) (string, error) {
			return "", errors.New("missing")
		},
		Stdin:      &bytes.Buffer{},
		Stdout:     &bytes.Buffer{},
		Stderr:     stderr,
		GOOS:       "linux",
		IsTerminal: func() bool { return false },
	})

	err := installer.EnsureGhInstalled(context.Background())
	require.Error(t, err)
	require.Contains(t, stderr.String(), "GitHub CLI install options")
}

func TestEnsureGhAuthenticatedPromptsLogin(t *testing.T) {
	runner := commandrunner.NewDummyCommandRunner()
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{Name: "gh", Args: []string{"auth", "status"}},
		Result:   commandrunner.Result{Stderr: []byte("not authenticated")},
		Err:      errors.New("status failed"),
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{Name: "gh", Args: []string{"auth", "login"}},
		Result:   commandrunner.Result{},
	})

	stdin := bytes.NewBufferString("y\n")
	installer := NewInstaller(Config{
		StreamRunner: runner,
		PreferTTY:    false,
		PreferTTYSet: true,
		LookPath: func(name string) (string, error) {
			if name == "gh" {
				return "/bin/gh", nil
			}
			return "", errors.New("missing")
		},
		Stdin:      stdin,
		Stdout:     &bytes.Buffer{},
		Stderr:     &bytes.Buffer{},
		GOOS:       "linux",
		IsTerminal: func() bool { return true },
	})

	err := installer.EnsureGhAuthenticated(context.Background())
	require.NoError(t, err)
	require.NoError(t, runner.VerifyDone())
}

func TestEnsureGhAuthenticatedSkipsLoginWithoutTTY(t *testing.T) {
	runner := commandrunner.NewDummyCommandRunner()
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{Name: "gh", Args: []string{"auth", "status"}},
		Result:   commandrunner.Result{Stderr: []byte("not authenticated")},
		Err:      errors.New("status failed"),
	})

	stderr := &bytes.Buffer{}
	installer := NewInstaller(Config{
		StreamRunner: runner,
		LookPath: func(name string) (string, error) {
			if name == "gh" {
				return "/bin/gh", nil
			}
			return "", errors.New("missing")
		},
		Stdin:      &bytes.Buffer{},
		Stdout:     &bytes.Buffer{},
		Stderr:     stderr,
		GOOS:       "linux",
		IsTerminal: func() bool { return false },
	})

	err := installer.EnsureGhAuthenticated(context.Background())
	require.NoError(t, err)
	require.Contains(t, stderr.String(), "Skipping 'gh auth login' because no TTY is available")
	require.NoError(t, runner.VerifyDone())
}

func TestEnsureGitInstalledUsesAptWhenAvailable(t *testing.T) {
	runner := commandrunner.NewDummyCommandRunner()
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{Name: "bash", Args: []string{"-c", gitAptInstallCommand}},
		Result:   commandrunner.Result{},
	})

	stdin := bytes.NewBufferString("y\n")
	installer := NewInstaller(Config{
		StreamRunner: runner,
		PreferTTY:    false,
		PreferTTYSet: true,
		LookPath: func(name string) (string, error) {
			switch name {
			case "git":
				return "", errors.New("missing")
			case "apt-get":
				return "/usr/bin/apt-get", nil
			default:
				return "", errors.New("missing")
			}
		},
		Stdin:      stdin,
		Stdout:     &bytes.Buffer{},
		Stderr:     &bytes.Buffer{},
		GOOS:       "linux",
		IsTerminal: func() bool { return true },
	})

	err := installer.EnsureGitInstalled(context.Background())
	require.NoError(t, err)
	require.NoError(t, runner.VerifyDone())
}

func TestEnsureGitInstalledNonInteractive(t *testing.T) {
	stderr := &bytes.Buffer{}
	installer := NewInstaller(Config{
		StreamRunner: commandrunner.NewDummyCommandRunner(),
		LookPath: func(name string) (string, error) {
			return "", errors.New("missing")
		},
		Stdin:      &bytes.Buffer{},
		Stdout:     &bytes.Buffer{},
		Stderr:     stderr,
		GOOS:       "linux",
		IsTerminal: func() bool { return false },
	})

	err := installer.EnsureGitInstalled(context.Background())
	require.Error(t, err)
	require.Contains(t, stderr.String(), "Git install options")
}

func TestEnsureGlabInstalledUsesAptWhenAvailable(t *testing.T) {
	runner := commandrunner.NewDummyCommandRunner()
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{Name: "bash", Args: []string{"-c", glabAptInstallCommand}},
		Result:   commandrunner.Result{},
	})

	stdin := bytes.NewBufferString("y\n")
	installer := NewInstaller(Config{
		StreamRunner: runner,
		PreferTTY:    false,
		PreferTTYSet: true,
		LookPath: func(name string) (string, error) {
			switch name {
			case "glab":
				return "", errors.New("missing")
			case "apt":
				return "/usr/bin/apt", nil
			default:
				return "", errors.New("missing")
			}
		},
		Stdin:      stdin,
		Stdout:     &bytes.Buffer{},
		Stderr:     &bytes.Buffer{},
		GOOS:       "linux",
		IsTerminal: func() bool { return true },
	})

	err := installer.EnsureGlabInstalled(context.Background())
	require.NoError(t, err)
	require.NoError(t, runner.VerifyDone())
}

func TestEnsureGlabInstalledErrorsWithoutSupportedLinuxManager(t *testing.T) {
	runner := commandrunner.NewDummyCommandRunner()

	stdin := bytes.NewBufferString("y\n")
	stderr := &bytes.Buffer{}
	installer := NewInstaller(Config{
		StreamRunner: runner,
		PreferTTY:    false,
		PreferTTYSet: true,
		LookPath: func(name string) (string, error) {
			switch name {
			case "glab":
				return "", errors.New("missing")
			case "yum":
				return "/usr/bin/yum", nil
			default:
				return "", errors.New("missing")
			}
		},
		Stdin:      stdin,
		Stdout:     &bytes.Buffer{},
		Stderr:     stderr,
		GOOS:       "linux",
		IsTerminal: func() bool { return true },
	})

	err := installer.EnsureGlabInstalled(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "no supported package manager found for glab install")
	require.Contains(t, stderr.String(), "GitLab CLI install options:")
	require.NoError(t, runner.VerifyDone())
}

func TestEnsureGlabAuthenticatedPromptsLogin(t *testing.T) {
	runner := commandrunner.NewDummyCommandRunner()
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{Name: "glab", Args: []string{"auth", "status"}},
		Result:   commandrunner.Result{Stderr: []byte("not authenticated")},
		Err:      errors.New("status failed"),
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{Name: "glab", Args: []string{"auth", "login"}},
		Result:   commandrunner.Result{},
	})

	stdin := bytes.NewBufferString("y\n")
	installer := NewInstaller(Config{
		StreamRunner: runner,
		PreferTTY:    false,
		PreferTTYSet: true,
		LookPath: func(name string) (string, error) {
			if name == "glab" {
				return "/bin/glab", nil
			}
			return "", errors.New("missing")
		},
		Stdin:      stdin,
		Stdout:     &bytes.Buffer{},
		Stderr:     &bytes.Buffer{},
		GOOS:       "linux",
		IsTerminal: func() bool { return true },
	})

	err := installer.EnsureGlabAuthenticated(context.Background())
	require.NoError(t, err)
	require.NoError(t, runner.VerifyDone())
}

func TestEnsureGlabAuthenticatedSkipsLoginWithoutTTY(t *testing.T) {
	runner := commandrunner.NewDummyCommandRunner()
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{Name: "glab", Args: []string{"auth", "status"}},
		Result:   commandrunner.Result{Stderr: []byte("not authenticated")},
		Err:      errors.New("status failed"),
	})

	stderr := &bytes.Buffer{}
	installer := NewInstaller(Config{
		StreamRunner: runner,
		LookPath: func(name string) (string, error) {
			if name == "glab" {
				return "/bin/glab", nil
			}
			return "", errors.New("missing")
		},
		Stdin:      &bytes.Buffer{},
		Stdout:     &bytes.Buffer{},
		Stderr:     stderr,
		GOOS:       "linux",
		IsTerminal: func() bool { return false },
	})

	err := installer.EnsureGlabAuthenticated(context.Background())
	require.NoError(t, err)
	require.Contains(t, stderr.String(), "Skipping 'glab auth login' because no TTY is available")
	require.NoError(t, runner.VerifyDone())
}
