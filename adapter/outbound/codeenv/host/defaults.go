package host

import (
	"github.com/bentos-lab/peer/adapter/outbound/commandrunner"
	"github.com/bentos-lab/peer/usecase"
)

type hostDefaults struct {
	runner      commandrunner.Runner
	agentRunner commandrunner.StreamRunner
	getwd       func() (string, error)
	makeTempDir func() (string, error)
	logger      usecase.Logger
}
