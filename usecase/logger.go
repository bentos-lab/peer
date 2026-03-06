package usecase

// Logger is the logging contract used by usecases and adapters.
type Logger interface {
	// Debugf writes a debug-level message.
	Debugf(format string, args ...any)
	// Infof writes an info-level message.
	Infof(format string, args ...any)
	// Warnf writes a warning-level message.
	Warnf(format string, args ...any)
	// Errorf writes an error-level message.
	Errorf(format string, args ...any)
}
