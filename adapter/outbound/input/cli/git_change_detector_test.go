package cli

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGitChangeDetector_ListParsesOutput(t *testing.T) {
	var calledArgs [][]string
	detector := newGitChangeDetectorWithRunner(func(_ context.Context, args ...string) ([]byte, error) {
		copied := make([]string, len(args))
		copy(copied, args)
		calledArgs = append(calledArgs, copied)
		switch args[0] {
		case "diff":
			return []byte("a.go\nb.go\n"), nil
		case "ls-files":
			return []byte("c.go\n"), nil
		default:
			return []byte(""), nil
		}
	})

	staged, err := detector.ListStaged(context.Background())
	require.NoError(t, err)
	require.Equal(t, []string{"a.go", "b.go"}, staged)

	unstaged, err := detector.ListUnstaged(context.Background())
	require.NoError(t, err)
	require.Equal(t, []string{"a.go", "b.go"}, unstaged)

	untracked, err := detector.ListUntracked(context.Background())
	require.NoError(t, err)
	require.Equal(t, []string{"c.go"}, untracked)

	require.Equal(t, []string{"diff", "--cached", "--name-only", "--diff-filter=ACMRTUXB"}, calledArgs[0])
	require.Equal(t, []string{"diff", "--name-only", "--diff-filter=ACMRTUXB"}, calledArgs[1])
	require.Equal(t, []string{"ls-files", "--others", "--exclude-standard"}, calledArgs[2])
}

func TestGitChangeDetector_ListPropagatesErrors(t *testing.T) {
	detector := newGitChangeDetectorWithRunner(func(_ context.Context, _ ...string) ([]byte, error) {
		return nil, errors.New("git failed")
	})

	_, err := detector.ListStaged(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "git failed")
}
