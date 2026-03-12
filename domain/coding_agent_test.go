package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCodingAgentRunOptionsZeroValue(t *testing.T) {
	var options CodingAgentRunOptions

	require.Equal(t, "", options.Provider)
	require.Equal(t, "", options.Model)
	require.Equal(t, "", options.SessionID)
}

func TestCodingAgentSetupOptionsZeroValue(t *testing.T) {
	var options CodingAgentSetupOptions

	require.Equal(t, "", options.Agent)
	require.Equal(t, "", options.Ref)
}

func TestCodingAgentRunResultZeroValue(t *testing.T) {
	var result CodingAgentRunResult

	require.Equal(t, "", result.Text)
	require.Equal(t, "", result.SessionID)
}
