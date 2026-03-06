package wiring

import (
	"testing"

	"bentos-backend/config"
	"bentos-backend/shared/logger/stdlogger"
	"github.com/stretchr/testify/require"
)

func TestLoggingResolveLogLevelUsesConfigWhenOverrideIsEmpty(t *testing.T) {
	level, err := resolveLogLevel(config.Config{LogLevel: "warning"}, "")
	require.NoError(t, err)
	require.Equal(t, stdlogger.LevelWarning, level)
}

func TestLoggingResolveLogLevelUsesOverrideWhenProvided(t *testing.T) {
	level, err := resolveLogLevel(config.Config{LogLevel: "warning"}, "error")
	require.NoError(t, err)
	require.Equal(t, stdlogger.LevelError, level)
}

func TestLoggingResolveLogLevelFallsBackToInfoWhenEmpty(t *testing.T) {
	level, err := resolveLogLevel(config.Config{LogLevel: ""}, "")
	require.NoError(t, err)
	require.Equal(t, stdlogger.LevelInfo, level)
}

func TestLoggingResolveLogLevelRejectsInvalidValue(t *testing.T) {
	_, err := resolveLogLevel(config.Config{LogLevel: "verbose"}, "")
	require.Error(t, err)
	require.ErrorContains(t, err, "invalid log level")
}

func TestLoggingBuildLoggerRejectsInvalidOverride(t *testing.T) {
	_, err := BuildLogger(config.Config{LogLevel: "info"}, "verbose")
	require.Error(t, err)
	require.ErrorContains(t, err, "invalid log level")
}
