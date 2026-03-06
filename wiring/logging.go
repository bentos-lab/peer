package wiring

import (
	"fmt"
	"log"
	"strings"

	"bentos-backend/config"
	stdlogger "bentos-backend/shared/logger/stdlogger"
	"bentos-backend/usecase"
)

// resolveLogLevel determines the effective log level from override and config.
func resolveLogLevel(cfg config.Config, logLevelOverride string) (stdlogger.Level, error) {
	candidate := strings.TrimSpace(cfg.LogLevel)
	if strings.TrimSpace(logLevelOverride) != "" {
		candidate = strings.TrimSpace(logLevelOverride)
	}
	if candidate == "" {
		candidate = string(stdlogger.LevelInfo)
	}

	level, ok := stdlogger.ParseLevel(candidate)
	if !ok {
		return "", fmt.Errorf("invalid log level %q: allowed values are trace, debug, info, warning, error, silence", candidate)
	}
	return level, nil
}

// buildLogger creates a logger with level resolved from config and optional override.
func buildLogger(cfg config.Config, logLevelOverride string) (usecase.Logger, error) {
	level, err := resolveLogLevel(cfg, logLevelOverride)
	if err != nil {
		return nil, err
	}

	return stdlogger.NewWithLevel(log.New(log.Writer(), log.Prefix(), log.Flags()), level), nil
}

// BuildLogger creates a logger with level resolved from config and optional override.
func BuildLogger(cfg config.Config, logLevelOverride string) (usecase.Logger, error) {
	return buildLogger(cfg, logLevelOverride)
}
