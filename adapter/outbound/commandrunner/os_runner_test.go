package commandrunner

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOSCommandRunner_RunSuccess(t *testing.T) {
	runner := NewOSCommandRunner()

	result, err := runner.Run(context.Background(), "sh", "-c", "printf 'out'; printf 'err' 1>&2")
	require.NoError(t, err)
	require.Equal(t, "out", string(result.Stdout))
	require.Equal(t, "err", string(result.Stderr))
}

func TestOSCommandRunner_RunFailure(t *testing.T) {
	runner := NewOSCommandRunner()

	result, err := runner.Run(context.Background(), "sh", "-c", "printf 'boom' 1>&2; exit 7")
	require.Error(t, err)
	require.Contains(t, string(result.Stderr), "boom")
}
