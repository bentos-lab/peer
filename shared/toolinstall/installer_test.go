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
