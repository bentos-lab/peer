package host

import (
	"context"

	"github.com/bentos-lab/peer/adapter/outbound/commandrunner"
	"github.com/bentos-lab/peer/domain"
	"github.com/bentos-lab/peer/usecase"
	"github.com/bentos-lab/peer/usecase/contracts"
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

// NewFactory creates a host code environment factory with injected dependencies.
func NewFactory(cfg FactoryConfig) *Factory {
	defaults := resolveHostDefaults(
		cfg.Runner,
		cfg.AgentRunner,
		cfg.Getwd,
		cfg.MakeTempDir,
		cfg.Logger,
	)
	return &Factory{
		runner:      defaults.runner,
		agentRunner: defaults.agentRunner,
		getwd:       defaults.getwd,
		makeTempDir: defaults.makeTempDir,
		logger:      defaults.logger,
	}
}

// New creates a new host-backed environment for one execution request.
func (f *Factory) New(ctx context.Context, opts domain.CodeEnvironmentInitOptions) (contracts.CodeEnvironment, error) {
	environment := NewHostCodeEnvironment(HostCodeEnvironmentConfig{
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
