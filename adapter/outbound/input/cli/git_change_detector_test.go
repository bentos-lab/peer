package cli

import (
	"context"
	"errors"
	"testing"

	"bentos-backend/adapter/outbound/commandrunner"
	"github.com/stretchr/testify/require"
)

func TestGitChangeDetector_ListParsesOutput(t *testing.T) {
	runner := commandrunner.NewDummyCommandRunner()
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{Name: "git", Args: []string{"diff", "--cached", "--name-only", "--diff-filter=ACMRTUXB"}},
		Result:   commandrunner.Result{Stdout: []byte("a.go\nb.go\n")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{Name: "git", Args: []string{"diff", "--name-only", "--diff-filter=ACMRTUXB"}},
		Result:   commandrunner.Result{Stdout: []byte("a.go\nb.go\n")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{Name: "git", Args: []string{"ls-files", "--others", "--exclude-standard"}},
		Result:   commandrunner.Result{Stdout: []byte("c.go\n")},
	})
	detector := newGitChangeDetectorWithRunner(runner)

	staged, err := detector.ListStaged(context.Background())
	require.NoError(t, err)
	require.Equal(t, []string{"a.go", "b.go"}, staged)

	unstaged, err := detector.ListUnstaged(context.Background())
	require.NoError(t, err)
	require.Equal(t, []string{"a.go", "b.go"}, unstaged)

	untracked, err := detector.ListUntracked(context.Background())
	require.NoError(t, err)
	require.Equal(t, []string{"c.go"}, untracked)

	require.Equal(t, []string{"diff", "--cached", "--name-only", "--diff-filter=ACMRTUXB"}, runner.Calls()[0].Args)
	require.Equal(t, []string{"diff", "--name-only", "--diff-filter=ACMRTUXB"}, runner.Calls()[1].Args)
	require.Equal(t, []string{"ls-files", "--others", "--exclude-standard"}, runner.Calls()[2].Args)
	require.NoError(t, runner.VerifyDone())
}

func TestGitChangeDetector_ListPropagatesErrors(t *testing.T) {
	runner := commandrunner.NewDummyCommandRunner()
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{Name: "git", Args: []string{"diff", "--cached", "--name-only", "--diff-filter=ACMRTUXB"}},
		Result:   commandrunner.Result{Stderr: []byte("fatal")},
		Err:      errors.New("exit status 1"),
	})
	detector := newGitChangeDetectorWithRunner(runner)

	_, err := detector.ListStaged(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "git diff --cached --name-only --diff-filter=ACMRTUXB failed")
	require.Contains(t, err.Error(), "fatal")
	require.NoError(t, runner.VerifyDone())
}
