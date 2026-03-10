package cli

import (
	"testing"

	stdlogger "bentos-backend/shared/logger/stdlogger"
	"github.com/stretchr/testify/require"
)

func TestLogLevelOverrideFromVerbosity(t *testing.T) {
	testCases := []struct {
		name      string
		verbosity int
		expected  string
	}{
		{name: "zero", verbosity: 0, expected: ""},
		{name: "negative", verbosity: -1, expected: ""},
		{name: "info", verbosity: 1, expected: string(stdlogger.LevelInfo)},
		{name: "debug", verbosity: 2, expected: string(stdlogger.LevelDebug)},
		{name: "trace", verbosity: 3, expected: string(stdlogger.LevelTrace)},
		{name: "clamped", verbosity: 4, expected: string(stdlogger.LevelTrace)},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			require.Equal(t, testCase.expected, LogLevelOverrideFromVerbosity(testCase.verbosity))
		})
	}
}
