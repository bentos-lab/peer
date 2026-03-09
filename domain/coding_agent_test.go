package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCodingAgentRunOptionsZeroValue(t *testing.T) {
	var options CodingAgentRunOptions

	require.Equal(t, "", options.Provider)
	require.Equal(t, "", options.Model)
}

func TestCodingAgentRunResultZeroValue(t *testing.T) {
	var result CodingAgentRunResult

	require.Equal(t, "", result.Text)
}
