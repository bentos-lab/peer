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

const configPath = ".autogit/config.toml"

// Loader reads repo-scoped custom recipe configuration and guidance.
type Loader struct {
	readOnlySanitizer  usecase.SafetySanitizer
	readWriteSanitizer usecase.SafetySanitizer
	logger             usecase.Logger
}

// NewLoader creates a custom recipe loader.
func NewLoader(readOnlySanitizer usecase.SafetySanitizer, readWriteSanitizer usecase.SafetySanitizer, logger usecase.Logger) (*Loader, error) {
	if readOnlySanitizer == nil || readWriteSanitizer == nil {
		return nil, fmt.Errorf("custom recipe sanitizers must not be nil")
	}
	if logger == nil {
		logger = stdlogger.Nop()
	}
	return &Loader{
		readOnlySanitizer:  readOnlySanitizer,
		readWriteSanitizer: readWriteSanitizer,
		logger:             logger,
	}, nil
}

// Load returns the custom recipe loaded from the repo workspace.
func (l *Loader) Load(ctx context.Context, env uccontracts.CodeEnvironment, headRef string) (domain.CustomRecipe, error) {
	if env == nil {
		return domain.CustomRecipe{}, fmt.Errorf("code environment is required")
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
		return domain.CustomRecipe{}, nil
	}

	var parsed recipeConfig
	if err := toml.Unmarshal([]byte(rawConfig), &parsed); err != nil {
		l.logger.Warnf("Custom recipe config is invalid: %v", err)
		return domain.CustomRecipe{}, nil
	}

	recipe := domain.CustomRecipe{
		ReviewEnabled:                 parsed.Review.Enabled,
		ReviewSuggestions:             parsed.Review.Suggestions,
		OverviewEnabled:               parsed.Overview.Enabled,
		OverviewIssueAlignmentEnabled: parsed.Overview.IssueAlignment.Enabled,
		AutoreplyEnabled:              parsed.Autoreply.Enabled,
		AutogenEnabled:                parsed.Autogen.Enabled,
	}

	reviewRuleset, missingPath, err := l.readAndSanitize(ctx, env, headRef, parsed.Review.Ruleset, l.readOnlySanitizer)
	if err != nil {
		return domain.CustomRecipe{}, err
	}
	if missingPath != "" {
		recipe.MissingPaths = append(recipe.MissingPaths, missingPath)
	}
	recipe.ReviewRuleset = reviewRuleset

	overviewGuidance, missingPath, err := l.readAndSanitize(ctx, env, headRef, parsed.Overview.ExtraGuidance, l.readOnlySanitizer)
	if err != nil {
		return domain.CustomRecipe{}, err
	}
	if missingPath != "" {
		recipe.MissingPaths = append(recipe.MissingPaths, missingPath)
	}
	recipe.OverviewGuidance = overviewGuidance

	autoreplyGuidance, missingPath, err := l.readAndSanitize(ctx, env, headRef, parsed.Autoreply.ExtraGuidance, l.readOnlySanitizer)
	if err != nil {
		return domain.CustomRecipe{}, err
	}
	if missingPath != "" {
		recipe.MissingPaths = append(recipe.MissingPaths, missingPath)
	}
	recipe.AutoreplyGuidance = autoreplyGuidance

	autogenGuidance, missingPath, err := l.readAndSanitize(ctx, env, headRef, parsed.Autogen.ExtraGuidance, l.readWriteSanitizer)
	if err != nil {
		return domain.CustomRecipe{}, err
	}
	if missingPath != "" {
		recipe.MissingPaths = append(recipe.MissingPaths, missingPath)
	}
	recipe.AutogenGuidance = autogenGuidance

	return recipe, nil
}

type recipeConfig struct {
	Review    recipeReviewConfig    `toml:"review"`
	Overview  recipeOverviewConfig  `toml:"overview"`
	Autoreply recipeAutoreplyConfig `toml:"autoreply"`
	Autogen   recipeAutogenConfig   `toml:"autogen"`
}

type recipeReviewConfig struct {
	Enabled     *bool  `toml:"enabled"`
	Ruleset     string `toml:"ruleset"`
	Suggestions *bool  `toml:"suggestions"`
}

type recipeAutoreplyConfig struct {
	Enabled       *bool  `toml:"enabled"`
	ExtraGuidance string `toml:"extra_guidance"`
}

type recipeOverviewConfig struct {
	Enabled        *bool                      `toml:"enabled"`
	ExtraGuidance  string                     `toml:"extra_guidance"`
	IssueAlignment recipeIssueAlignmentConfig `toml:"issue_alignment"`
}

type recipeIssueAlignmentConfig struct {
	Enabled *bool `toml:"enabled"`
}

type recipeAutogenConfig struct {
	Enabled       *bool  `toml:"enabled"`
	ExtraGuidance string `toml:"extra_guidance"`
}
