package stdlogger

// noopLogger discards all log events.
type noopLogger struct{}

// Nop returns a logger that drops all log events.
func Nop() *noopLogger {
	return &noopLogger{}
}

// Debugf discards debug-level log messages.
func (*noopLogger) Debugf(_ string, _ ...any) {}

// Infof discards info-level log messages.
func (*noopLogger) Infof(_ string, _ ...any) {}

// Warnf discards warning-level log messages.
func (*noopLogger) Warnf(_ string, _ ...any) {}

// Errorf discards error-level log messages.
func (*noopLogger) Errorf(_ string, _ ...any) {}
