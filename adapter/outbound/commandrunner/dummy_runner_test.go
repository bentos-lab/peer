package commandrunner

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDummyCommandRunner_RunOrderedQueue(t *testing.T) {
	runner := NewDummyCommandRunner()
	runner.Enqueue(CommandStep{
		Expected: CommandCall{Name: "git", Args: []string{"status", "--short"}},
		Result:   Result{Stdout: []byte("M file.go\n")},
	})

	result, err := runner.Run(context.Background(), "git", "status", "--short")
	require.NoError(t, err)
	require.Equal(t, "M file.go\n", string(result.Stdout))
	require.NoError(t, runner.VerifyDone())
	require.Len(t, runner.Calls(), 1)
}

func TestDummyCommandRunner_RunMismatchedCommand(t *testing.T) {
	runner := NewDummyCommandRunner()
	runner.Enqueue(CommandStep{Expected: CommandCall{Name: "git", Args: []string{"status"}}})

	_, err := runner.Run(context.Background(), "git", "diff")
	require.Error(t, err)
	require.Contains(t, err.Error(), "unexpected command call")
}

func TestDummyCommandRunner_RunReturnsScriptedError(t *testing.T) {
	runner := NewDummyCommandRunner()
	runner.Enqueue(CommandStep{
		Expected: CommandCall{Name: "gh", Args: []string{"auth", "status"}},
		Err:      errors.New("exit status 1"),
	})

	_, err := runner.Run(context.Background(), "gh", "auth", "status")
	require.EqualError(t, err, "exit status 1")
	require.NoError(t, runner.VerifyDone())
}

func TestDummyCommandRunner_VerifyDoneDetectsLeftoverSteps(t *testing.T) {
	runner := NewDummyCommandRunner()
	runner.Enqueue(CommandStep{Expected: CommandCall{Name: "git", Args: []string{"status"}}})

	err := runner.VerifyDone()
	require.Error(t, err)
	require.Contains(t, err.Error(), "unconsumed command steps")
}
