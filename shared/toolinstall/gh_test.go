package toolinstall

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"bentos-backend/adapter/outbound/commandrunner"

	"github.com/stretchr/testify/require"
)

func TestEnsureGhInstalledUsesAptWhenAvailable(t *testing.T) {
	runner := commandrunner.NewDummyCommandRunner()
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{Name: "bash", Args: []string{"-c", ghAptInstallCommand}},
		Result:   commandrunner.Result{},
	})

	preferTTY := false
	stdin := bytes.NewBufferString("y\n")
	installer := NewGhInstaller(&Deps{
		StreamRunner: runner,
		PreferTTY:    &preferTTY,
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
	installer := NewGhInstaller(&Deps{
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

	preferTTY := false
	stdin := bytes.NewBufferString("y\n")
	installer := NewGhInstaller(&Deps{
		StreamRunner: runner,
		PreferTTY:    &preferTTY,
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
	installer := NewGhInstaller(&Deps{
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
