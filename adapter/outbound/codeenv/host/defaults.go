package host

import (
	"bentos-backend/adapter/outbound/commandrunner"
	"bentos-backend/usecase"
)

type hostDefaults struct {
	runner      commandrunner.Runner
	agentRunner commandrunner.StreamRunner
	getwd       func() (string, error)
	makeTempDir func() (string, error)
	logger      usecase.Logger
}
