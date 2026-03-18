package toolinstall

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"bentos-backend/adapter/outbound/commandrunner"

	"github.com/stretchr/testify/require"
)

func TestEnsureGitInstalledUsesAptWhenAvailable(t *testing.T) {
	runner := commandrunner.NewDummyCommandRunner()
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{Name: "bash", Args: []string{"-c", gitAptInstallCommand}},
		Result:   commandrunner.Result{},
	})

	preferTTY := false
	stdin := bytes.NewBufferString("y\n")
	installer := NewGitInstaller(&Deps{
		StreamRunner: runner,
		PreferTTY:    &preferTTY,
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
	installer := NewGitInstaller(&Deps{
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
