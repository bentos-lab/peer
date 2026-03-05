package usecase

// Logger is the logging contract used by usecases and adapters.
type Logger interface {
	// Debugf writes a debug-level message.
	Debugf(format string, args ...any)
	// Infof writes an info-level message.
	Infof(format string, args ...any)
	// Errorf writes an error-level message.
	Errorf(format string, args ...any)
}

type noopLogger struct{}

// Debugf writes a debug-level message. This noop implementation discards logs.
func (noopLogger) Debugf(_ string, _ ...any) {}

// Infof writes an info-level message. This noop implementation discards logs.
func (noopLogger) Infof(_ string, _ ...any) {}

// Errorf writes an error-level message. This noop implementation discards logs.
func (noopLogger) Errorf(_ string, _ ...any) {}

// NopLogger is a safe default logger that does nothing.
var NopLogger Logger = noopLogger{}
