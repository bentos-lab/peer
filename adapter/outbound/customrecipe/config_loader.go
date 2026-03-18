package customrecipe

import (
	"context"
	"fmt"
	"strings"

	"github.com/bentos-lab/peer/domain"
	"github.com/bentos-lab/peer/shared/logger/stdlogger"
	"github.com/bentos-lab/peer/usecase"
	uccontracts "github.com/bentos-lab/peer/usecase/contracts"
)

// ConfigLoader reads enabled toggles from the repo-scoped recipe config.
type ConfigLoader struct {
	factory uccontracts.CodeEnvironmentFactory
	logger  usecase.Logger
}

// NewConfigLoader creates a custom recipe config loader.
func NewConfigLoader(factory uccontracts.CodeEnvironmentFactory, logger usecase.Logger) (*ConfigLoader, error) {
	if factory == nil {
		return nil, fmt.Errorf("code environment factory is required")
	}
	if logger == nil {
		logger = stdlogger.Nop()
	}
	return &ConfigLoader{
		factory: factory,
		logger:  logger,
	}, nil
}

// Load returns enabled toggles from the repo-scoped recipe configuration.
func (l *ConfigLoader) Load(ctx context.Context, repoURL string, headRef string) (domain.CustomRecipe, error) {
	if strings.TrimSpace(repoURL) == "" {
		return domain.CustomRecipe{}, nil
	}
	headRef = strings.TrimSpace(headRef)
	if headRef == "" {
		headRef = "HEAD"
	}

	recipe := loadEnvRecipeConfigDefaults(l.logger)

	env, err := l.factory.New(ctx, domain.CodeEnvironmentInitOptions{RepoURL: repoURL})
	if err != nil {
		return domain.CustomRecipe{}, err
	}
	defer func() {
		if cleanupErr := env.Cleanup(ctx); cleanupErr != nil {
			l.logger.Warnf("Failed to cleanup code environment: %v", cleanupErr)
		}
	}()

	rawConfig, found, err := env.ReadFile(ctx, configPath, headRef)
	if err != nil {
		return domain.CustomRecipe{}, err
	}
	if !found || strings.TrimSpace(rawConfig) == "" {
		return recipe, nil
	}

	parsed, ok := parseRecipeConfig(rawConfig, l.logger)
	if !ok {
		return recipe, nil
	}
	recipe = applyRecipeOverrides(recipe, parsed)

	return recipe, nil
}

func loadEnvRecipeConfigDefaults(logger usecase.Logger) domain.CustomRecipe {
	return LoadDefaultsFromEnv(logger)
}
