package customrecipe

import (
	"context"
	"fmt"
	"strings"

	"bentos-backend/domain"
	"bentos-backend/shared/logger/stdlogger"
	"bentos-backend/usecase"
	uccontracts "bentos-backend/usecase/contracts"

	"github.com/pelletier/go-toml/v2"
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
		return domain.CustomRecipe{}, nil
	}

	var parsed recipeConfig
	if err := toml.Unmarshal([]byte(rawConfig), &parsed); err != nil {
		l.logger.Warnf("Custom recipe config is invalid: %v", err)
		return domain.CustomRecipe{}, nil
	}

	return domain.CustomRecipe{
		ReviewEnabled:                 parsed.Review.Enabled,
		ReviewSuggestions:             parsed.Review.Suggestions,
		OverviewEnabled:               parsed.Overview.Enabled,
		OverviewIssueAlignmentEnabled: parsed.Overview.IssueAlignment.Enabled,
		AutoreplyEnabled:              parsed.Autoreply.Enabled,
		AutogenEnabled:                parsed.Autogen.Enabled,
	}, nil
}
