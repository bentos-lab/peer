package stdlogger

import (
	"bytes"
	"errors"
	"log"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoggerInfofWritesFormattedMessage(t *testing.T) {
	var buffer bytes.Buffer
	logger := New(log.New(&buffer, "", 0))

	logger.Infof("Review started for ID %s with count %d.", "r1", 12)

	output := buffer.String()
	require.Contains(t, output, "[INFO] Review started for ID r1 with count 12.")
}

func TestLoggerDebugfWritesFormattedMessage(t *testing.T) {
	var buffer bytes.Buffer
	logger := NewWithLevel(log.New(&buffer, "", 0), LevelDebug)

	logger.Debugf("Using local provider.")

	output := buffer.String()
	require.Contains(t, output, "[DEBUG] Using local provider.")
}

func TestLoggerTracefWritesFormattedMessage(t *testing.T) {
	var buffer bytes.Buffer
	logger := NewWithLevel(log.New(&buffer, "", 0), LevelTrace)

	logger.Tracef("llm output: %s", "{\"ok\":true}")

	output := buffer.String()
	require.Contains(t, output, "[TRACE] llm output: {\"ok\":true}")
}

func TestLoggerWarnfWritesFormattedMessage(t *testing.T) {
	var buffer bytes.Buffer
	logger := New(log.New(&buffer, "", 0))

	logger.Warnf("Skipped one comment because the anchor is invalid.")

	output := buffer.String()
	require.Contains(t, output, "[WARN] Skipped one comment because the anchor is invalid.")
}

func TestLoggerErrorfWritesFormattedMessage(t *testing.T) {
	var buffer bytes.Buffer
	logger := New(log.New(&buffer, "", 0))
	err := errors.New("boom")

	logger.Errorf("stage failed: %v", err)

	output := buffer.String()
	require.Contains(t, output, "[ERROR] stage failed: boom")
}

func TestLoggerFiltersEventsByLevel(t *testing.T) {
	var buffer bytes.Buffer
	logger := NewWithLevel(log.New(&buffer, "", 0), LevelWarning)

	logger.Debugf("debug detail")
	logger.Infof("review started")
	logger.Warnf("review warning")
	logger.Errorf("stage failed")

	output := buffer.String()
	require.NotContains(t, output, "[DEBUG] debug detail")
	require.NotContains(t, output, "[INFO] review started")
	require.Contains(t, output, "[WARN] review warning")
	require.Contains(t, output, "[ERROR] stage failed")
}

func TestLoggerDebugLevelIncludesDebugInfoWarnAndError(t *testing.T) {
	var buffer bytes.Buffer
	logger := NewWithLevel(log.New(&buffer, "", 0), LevelDebug)

	logger.Tracef("trace detail")
	logger.Debugf("debug detail")
	logger.Infof("info detail")
	logger.Warnf("warn detail")
	logger.Errorf("error detail")

	output := buffer.String()
	require.NotContains(t, output, "[TRACE] trace detail")
	require.Contains(t, output, "[DEBUG] debug detail")
	require.Contains(t, output, "[INFO] info detail")
	require.Contains(t, output, "[WARN] warn detail")
	require.Contains(t, output, "[ERROR] error detail")
}

func TestLoggerTraceLevelIncludesTraceDebugInfoWarnAndError(t *testing.T) {
	var buffer bytes.Buffer
	logger := NewWithLevel(log.New(&buffer, "", 0), LevelTrace)

	logger.Tracef("trace detail")
	logger.Debugf("debug detail")
	logger.Infof("info detail")
	logger.Warnf("warn detail")
	logger.Errorf("error detail")

	output := buffer.String()
	require.Contains(t, output, "[TRACE] trace detail")
	require.Contains(t, output, "[DEBUG] debug detail")
	require.Contains(t, output, "[INFO] info detail")
	require.Contains(t, output, "[WARN] warn detail")
	require.Contains(t, output, "[ERROR] error detail")
}

func TestLoggerInfoLevelIncludesInfoWarnAndError(t *testing.T) {
	var buffer bytes.Buffer
	logger := NewWithLevel(log.New(&buffer, "", 0), LevelInfo)

	logger.Debugf("debug detail")
	logger.Infof("info detail")
	logger.Warnf("warn detail")
	logger.Errorf("error detail")

	output := buffer.String()
	require.NotContains(t, output, "[DEBUG] debug detail")
	require.Contains(t, output, "[INFO] info detail")
	require.Contains(t, output, "[WARN] warn detail")
	require.Contains(t, output, "[ERROR] error detail")
}

func TestLoggerSilenceDropsAllEvents(t *testing.T) {
	var buffer bytes.Buffer
	logger := NewWithLevel(log.New(&buffer, "", 0), LevelSilence)

	logger.Tracef("trace detail")
	logger.Debugf("debug detail")
	logger.Infof("review started")
	logger.Warnf("review warning")
	logger.Errorf("stage failed")

	require.Empty(t, buffer.String())
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  Level
		ok    bool
	}{
		{name: "trace", input: "trace", want: LevelTrace, ok: true},
		{name: "debug", input: "DEBUG", want: LevelDebug, ok: true},
		{name: "info", input: "info", want: LevelInfo, ok: true},
		{name: "warning", input: "warning", want: LevelWarning, ok: true},
		{name: "warn alias", input: "warn", want: LevelWarning, ok: true},
		{name: "error", input: "error", want: LevelError, ok: true},
		{name: "silence", input: "silence", want: LevelSilence, ok: true},
		{name: "invalid", input: "verbose", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParseLevel(tt.input)
			require.Equal(t, tt.ok, ok)
			require.Equal(t, tt.want, got)
		})
	}
}
