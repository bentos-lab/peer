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

	preferTTY := false
	stdin := bytes.NewBufferString("y\n")
	installer := NewOpencodeInstaller(&Deps{
		StreamRunner: runner,
		PreferTTY:    &preferTTY,
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

	preferTTY := false
	stdin := bytes.NewBufferString("y\n")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	installer := NewOpencodeInstaller(&Deps{
		StreamRunner: runner,
		PreferTTY:    &preferTTY,
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
