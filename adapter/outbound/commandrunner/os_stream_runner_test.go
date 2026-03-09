package commandrunner

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOSStreamCommandRunner_RunStreamSuccess(t *testing.T) {
	runner := NewOSStreamCommandRunner()

	var chunks []StreamChunk
	result, err := runner.RunStream(context.Background(), func(chunk StreamChunk) {
		chunks = append(chunks, chunk)
	}, "sh", "-c", "printf 'out'; printf 'err' 1>&2")
	require.NoError(t, err)
	require.Equal(t, "out", string(result.Stdout))
	require.Equal(t, "err", string(result.Stderr))
	require.NotEmpty(t, chunks)
}

func TestOSStreamCommandRunner_RunStreamFailure(t *testing.T) {
	runner := NewOSStreamCommandRunner()

	result, err := runner.RunStream(context.Background(), nil, "sh", "-c", "printf 'boom' 1>&2; exit 7")
	require.Error(t, err)
	require.Contains(t, string(result.Stderr), "boom")
}
