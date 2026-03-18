package cli

import stdlogger "github.com/bentos-lab/peer/shared/logger/stdlogger"

// LogLevelOverrideFromVerbosity maps a verbosity count to a log level override.
func LogLevelOverrideFromVerbosity(verbosity int) string {
	switch {
	case verbosity <= 0:
		return string(stdlogger.LevelInfo)
	case verbosity == 1:
		return string(stdlogger.LevelDebug)
	default:
		return string(stdlogger.LevelTrace)
	}
}
