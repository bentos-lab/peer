package customrecipe

import (
	"context"
	"fmt"
	"strings"

	"bentos-backend/domain"
	"bentos-backend/shared/logger/stdlogger"
	"bentos-backend/usecase"
	uccontracts "bentos-backend/usecase/contracts"
)

// LoadDefaultsFromEnv returns custom recipe defaults from environment variables.
func LoadDefaultsFromEnv(logger usecase.Logger) domain.CustomRecipe {
	return buildRecipeFromDefaults(loadRecipeEnvDefaults(logger))
}

// OverrideFromCodeEnv overrides recipe values using .peer/config.toml if present.
func OverrideFromCodeEnv(
	ctx context.Context,
	env uccontracts.CodeEnvironment,
	headRef string,
	base domain.CustomRecipe,
	logger usecase.Logger,
) (domain.CustomRecipe, error) {
	if env == nil {
		return domain.CustomRecipe{}, fmt.Errorf("code environment is required")
	}
	if logger == nil {
		logger = stdlogger.Nop()
	}
	headRef = strings.TrimSpace(headRef)
	if headRef == "" {
		headRef = "HEAD"
	}

	rawConfig, found, err := env.ReadFile(ctx, configPath, headRef)
	if err != nil {
		return domain.CustomRecipe{}, err
	}
	if !found || strings.TrimSpace(rawConfig) == "" {
		return base, nil
	}

	parsed, ok := parseRecipeConfig(rawConfig, logger)
	if !ok {
		return base, nil
	}

	overridden := applyRecipeOverrides(base, parsed)
	return overridden, nil
}

func buildRecipeFromDefaults(defaults recipeEnvDefaults) domain.CustomRecipe {
	recipe := domain.CustomRecipe{
		ReviewEnabled:                 defaults.ReviewEnabled,
		ReviewSuggestions:             defaults.ReviewSuggestions,
		OverviewEnabled:               defaults.OverviewEnabled,
		OverviewIssueAlignmentEnabled: defaults.OverviewIssueAlignmentEnabled,
		ReplyCommentEnabled:           defaults.ReplyCommentEnabled,
		AutogenEnabled:                defaults.AutogenEnabled,
		AutogenDocs:                   defaults.AutogenDocs,
		AutogenTests:                  defaults.AutogenTests,
	}
	if defaults.ReviewEvents.Set {
		recipe.ReviewEvents = defaults.ReviewEvents.Values
	}
	if defaults.OverviewEvents.Set {
		recipe.OverviewEvents = defaults.OverviewEvents.Values
	}
	if defaults.ReplyCommentEvents.Set {
		recipe.ReplyCommentEvents = defaults.ReplyCommentEvents.Values
	}
	if defaults.ReplyCommentActions.Set {
		recipe.ReplyCommentActions = defaults.ReplyCommentActions.Values
	}
	if defaults.AutogenEvents.Set {
		recipe.AutogenEvents = defaults.AutogenEvents.Values
	}
	return recipe
}
