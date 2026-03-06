package stdlogger

import (
	"log"
	"strings"
)

// Level defines logger verbosity threshold.
type Level string

const (
	// LevelTrace logs all events.
	LevelTrace Level = "trace"
	// LevelDebug logs all events.
	LevelDebug Level = "debug"
	// LevelInfo logs info and error events.
	LevelInfo Level = "info"
	// LevelWarning logs only warning or higher events.
	LevelWarning Level = "warning"
	// LevelError logs only error events.
	LevelError Level = "error"
	// LevelSilence disables all log output.
	LevelSilence Level = "silence"
)

// Logger writes formatted logs using the standard library logger.
type Logger struct {
	std   *log.Logger
	level Level
}

// New creates a Logger backed by the given standard logger.
func New(std *log.Logger) *Logger {
	return NewWithLevel(std, LevelInfo)
}

// NewWithLevel creates a Logger backed by the given standard logger and level.
func NewWithLevel(std *log.Logger, level Level) *Logger {
	if std == nil {
		std = log.Default()
	}
	return &Logger{std: std, level: level}
}

// Infof writes an info-level message.
func (l *Logger) Infof(format string, args ...any) {
	l.logf("[INFO]", LevelInfo, format, args...)
}

// Warnf writes a warning-level message.
func (l *Logger) Warnf(format string, args ...any) {
	l.logf("[WARN]", LevelWarning, format, args...)
}

// Debugf writes a debug-level message.
func (l *Logger) Debugf(format string, args ...any) {
	l.logf("[DEBUG]", LevelDebug, format, args...)
}

// Errorf writes an error-level message.
func (l *Logger) Errorf(format string, args ...any) {
	l.logf("[ERROR]", LevelError, format, args...)
}

// ParseLevel validates and normalizes a log level string.
func ParseLevel(value string) (Level, bool) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case string(LevelTrace):
		return LevelTrace, true
	case string(LevelDebug):
		return LevelDebug, true
	case string(LevelInfo):
		return LevelInfo, true
	case "warn", string(LevelWarning):
		return LevelWarning, true
	case string(LevelError):
		return LevelError, true
	case string(LevelSilence):
		return LevelSilence, true
	default:
		return "", false
	}
}

func (l *Logger) logf(label string, eventLevel Level, format string, args ...any) {
	if !l.shouldLog(eventLevel) {
		return
	}

	l.std.Printf(label+" "+format, args...)
}

func (l *Logger) shouldLog(eventLevel Level) bool {
	switch l.level {
	case LevelTrace, LevelDebug:
		return eventLevel == LevelDebug || eventLevel == LevelInfo || eventLevel == LevelWarning || eventLevel == LevelError
	case LevelInfo:
		return eventLevel == LevelInfo || eventLevel == LevelWarning || eventLevel == LevelError
	case LevelWarning:
		return eventLevel == LevelWarning || eventLevel == LevelError
	case LevelError:
		return eventLevel == LevelError
	case LevelSilence:
		return false
	default:
		return eventLevel == LevelInfo || eventLevel == LevelWarning || eventLevel == LevelError
	}
}
