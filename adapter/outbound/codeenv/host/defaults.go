package host

import (
	"os"

	"bentos-backend/adapter/outbound/commandrunner"
	"bentos-backend/shared/logger/stdlogger"
	"bentos-backend/usecase"
)

type hostDefaults struct {
	runner      commandrunner.Runner
	agentRunner commandrunner.StreamRunner
	getwd       func() (string, error)
	makeTempDir func() (string, error)
	logger      usecase.Logger
}

func resolveHostDefaults(
	runner commandrunner.Runner,
	agentRunner commandrunner.StreamRunner,
	getwd func() (string, error),
	makeTempDir func() (string, error),
	logger usecase.Logger,
) hostDefaults {
	if runner == nil {
		runner = commandrunner.NewOSCommandRunner()
	}
	if agentRunner == nil {
		agentRunner = commandrunner.NewOSStreamCommandRunner()
	}
	if getwd == nil {
		getwd = os.Getwd
	}
	if makeTempDir == nil {
		makeTempDir = newHostCodeEnvironmentTempDirMaker(os.UserHomeDir)
	}
	if logger == nil {
		logger = stdlogger.Nop()
	}
	return hostDefaults{
		runner:      runner,
		agentRunner: agentRunner,
		getwd:       getwd,
		makeTempDir: makeTempDir,
		logger:      logger,
	}
}
