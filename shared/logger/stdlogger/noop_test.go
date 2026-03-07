package stdlogger

import "testing"

func TestNopLoggerAcceptsFormattedCalls(t *testing.T) {
	logger := Nop()

	logger.Tracef("trace %s %d", "value", 1)
	logger.Debugf("debug %s %d", "value", 1)
	logger.Infof("info %s %d", "value", 1)
	logger.Warnf("warn %s %d", "value", 1)
	logger.Errorf("error %s %d", "value", 1)
}
