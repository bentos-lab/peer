package codeenv

import (
	"context"
	"fmt"

	"bentos-backend/adapter/outbound/customrecipe"
	"bentos-backend/domain"
	"bentos-backend/shared/logger/stdlogger"
	"bentos-backend/usecase"
	uccontracts "bentos-backend/usecase/contracts"
)

// NewEnvironment creates a code environment and returns a cleanup function.
func NewEnvironment(
	ctx context.Context,
	factory uccontracts.CodeEnvironmentFactory,
	repoURL string,
) (uccontracts.CodeEnvironment, func(context.Context) error, error) {
	if factory == nil {
		return nil, nil, fmt.Errorf("code environment factory is required")
	}
	env, err := factory.New(ctx, domain.CodeEnvironmentInitOptions{RepoURL: repoURL})
	if err != nil {
		return nil, nil, err
	}
	cleanup := func(cleanupCtx context.Context) error {
		return env.Cleanup(cleanupCtx)
	}
	return env, cleanup, nil
}

// OverrideConfig overrides a base recipe using .peer/config.toml in the code environment.
func OverrideConfig(
	ctx context.Context,
	env uccontracts.CodeEnvironment,
	headRef string,
	base domain.CustomRecipe,
	logger usecase.Logger,
) (domain.CustomRecipe, error) {
	if logger == nil {
		logger = stdlogger.Nop()
	}
	return customrecipe.OverrideFromCodeEnv(ctx, env, headRef, base, logger)
}
