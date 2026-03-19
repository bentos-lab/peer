package cli

import (
	"testing"

	stdlogger "github.com/bentos-lab/peer/shared/logger/stdlogger"
	"github.com/stretchr/testify/require"
)

func TestLogLevelOverrideFromVerbosity(t *testing.T) {
	testCases := []struct {
		name      string
		verbosity int
		expected  string
	}{
		{name: "zero", verbosity: 0, expected: string(stdlogger.LevelWarning)},
		{name: "negative", verbosity: -1, expected: string(stdlogger.LevelWarning)},
		{name: "info", verbosity: 1, expected: string(stdlogger.LevelInfo)},
		{name: "debug", verbosity: 2, expected: string(stdlogger.LevelDebug)},
		{name: "trace", verbosity: 3, expected: string(stdlogger.LevelTrace)},
		{name: "trace_high", verbosity: 4, expected: string(stdlogger.LevelTrace)},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			require.Equal(t, testCase.expected, LogLevelOverrideFromVerbosity(testCase.verbosity))
		})
	}
}
