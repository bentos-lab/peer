package host

import (
	"context"
	"os"

	"bentos-backend/adapter/outbound/commandrunner"
	"bentos-backend/domain"
	"bentos-backend/shared/logger/stdlogger"
	"bentos-backend/usecase"
	"bentos-backend/usecase/contracts"
)

// FactoryConfig contains dependencies for creating prepared host environments.
type FactoryConfig struct {
	Runner      commandrunner.Runner
	AgentRunner commandrunner.StreamRunner
	Getwd       func() (string, error)
	MakeTempDir func() (string, error)
	Logger      usecase.Logger
}

// Factory creates host-backed code environments per request.
type Factory struct {
	runner      commandrunner.Runner
	agentRunner commandrunner.StreamRunner
	getwd       func() (string, error)
	makeTempDir func() (string, error)
	logger      usecase.Logger
}

// NewFactory creates a host code environment factory.
func NewFactory(logger usecase.Logger) *Factory {
	return NewFactoryWithConfig(FactoryConfig{
		Runner:      commandrunner.NewOSCommandRunner(),
		AgentRunner: commandrunner.NewOSStreamCommandRunner(),
		Getwd:       os.Getwd,
		MakeTempDir: newHostCodeEnvironmentTempDirMaker(os.UserHomeDir),
		Logger:      logger,
	})
}

// NewFactoryWithConfig creates a host code environment factory with injected dependencies.
func NewFactoryWithConfig(cfg FactoryConfig) *Factory {
	runner := cfg.Runner
	if runner == nil {
		runner = commandrunner.NewOSCommandRunner()
	}
	agentRunner := cfg.AgentRunner
	if agentRunner == nil {
		agentRunner = commandrunner.NewOSStreamCommandRunner()
	}
	getwd := cfg.Getwd
	if getwd == nil {
		getwd = os.Getwd
	}
	makeTempDir := cfg.MakeTempDir
	if makeTempDir == nil {
		makeTempDir = newHostCodeEnvironmentTempDirMaker(os.UserHomeDir)
	}
	logger := cfg.Logger
	if logger == nil {
		logger = stdlogger.Nop()
	}
	return &Factory{
		runner:      runner,
		agentRunner: agentRunner,
		getwd:       getwd,
		makeTempDir: makeTempDir,
		logger:      logger,
	}
}

// New creates a new host-backed environment for one execution request.
func (f *Factory) New(ctx context.Context, opts domain.CodeEnvironmentInitOptions) (contracts.CodeEnvironment, error) {
	environment := NewHostCodeEnvironmentWithConfig(HostCodeEnvironmentConfig{
		Runner:      f.runner,
		AgentRunner: f.agentRunner,
		Getwd:       f.getwd,
		MakeTempDir: f.makeTempDir,
		Logger:      f.logger,
	})
	if err := environment.prepareWorkspace(ctx, opts.RepoURL); err != nil {
		return nil, err
	}
	return environment, nil
}
