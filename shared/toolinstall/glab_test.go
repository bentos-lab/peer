package toolinstall

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"bentos-backend/adapter/outbound/commandrunner"

	"github.com/stretchr/testify/require"
)

func TestEnsureGlabInstalledUsesAptWhenAvailable(t *testing.T) {
	runner := commandrunner.NewDummyCommandRunner()
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{Name: "bash", Args: []string{"-c", glabAptInstallCommand}},
		Result:   commandrunner.Result{},
	})

	preferTTY := false
	stdin := bytes.NewBufferString("y\n")
	installer := NewGlabInstaller(&Deps{
		StreamRunner: runner,
		PreferTTY:    &preferTTY,
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

	preferTTY := false
	stdin := bytes.NewBufferString("y\n")
	stderr := &bytes.Buffer{}
	installer := NewGlabInstaller(&Deps{
		StreamRunner: runner,
		PreferTTY:    &preferTTY,
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

	preferTTY := false
	stdin := bytes.NewBufferString("y\n")
	installer := NewGlabInstaller(&Deps{
		StreamRunner: runner,
		PreferTTY:    &preferTTY,
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
	installer := NewGlabInstaller(&Deps{
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
