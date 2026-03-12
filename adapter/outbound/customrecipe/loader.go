package customrecipe

import (
	"context"
	"fmt"
	"path/filepath"
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
		ReviewSuggestions: parsed.Review.Suggestions,
		ReviewOverview:    parsed.Review.Overview.Enabled,
	}

	reviewRuleset, missingPath, err := l.readAndSanitize(ctx, env, headRef, parsed.Review.Ruleset, l.readOnlySanitizer)
	if err != nil {
		return domain.CustomRecipe{}, err
	}
	if missingPath != "" {
		recipe.MissingPaths = append(recipe.MissingPaths, missingPath)
	}
	recipe.ReviewRuleset = reviewRuleset

	overviewGuidance, missingPath, err := l.readAndSanitize(ctx, env, headRef, parsed.Review.Overview.ExtraGuidance, l.readOnlySanitizer)
	if err != nil {
		return domain.CustomRecipe{}, err
	}
	if missingPath != "" {
		recipe.MissingPaths = append(recipe.MissingPaths, missingPath)
	}
	recipe.ReviewOverviewGuidance = overviewGuidance

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

func (l *Loader) readAndSanitize(ctx context.Context, env uccontracts.CodeEnvironment, headRef string, rawPath string, sanitizer usecase.SafetySanitizer) (string, string, error) {
	path, err := resolveRecipePath(rawPath)
	if err != nil {
		l.logger.Warnf("Custom recipe path %q is invalid: %v", rawPath, err)
		return "", "", nil
	}
	if path == "" {
		return "", "", nil
	}

	fullPath := filepath.Join(".autogit", path)
	content, found, err := env.ReadFile(ctx, fullPath, headRef)
	if err != nil {
		return "", "", err
	}
	if !found {
		l.logger.Warnf("Custom recipe file %q was not found.", fullPath)
		return "", fullPath, nil
	}

	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return "", "", nil
	}
	result, err := sanitizer.Sanitize(ctx, trimmed)
	if err != nil {
		return "", "", err
	}
	if result.Status != domain.PromptSafetyStatusOK {
		l.logger.Warnf("Custom recipe file %q was rejected by sanitizer.", fullPath)
		return "", "", nil
	}
	return strings.TrimSpace(result.SanitizedPrompt), "", nil
}

func resolveRecipePath(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", nil
	}
	if filepath.IsAbs(trimmed) {
		return "", fmt.Errorf("path must be relative to .autogit")
	}
	cleaned := filepath.Clean(trimmed)
	if cleaned == "." {
		return "", fmt.Errorf("path must be a file")
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path must remain within .autogit")
	}
	return cleaned, nil
}

type recipeConfig struct {
	Review    recipeReviewConfig    `toml:"review"`
	Autoreply recipeAutoreplyConfig `toml:"autoreply"`
	Autogen   recipeAutogenConfig   `toml:"autogen"`
}

type recipeReviewConfig struct {
	Ruleset     string               `toml:"ruleset"`
	Suggestions *bool                `toml:"suggestions"`
	Overview    recipeOverviewConfig `toml:"overview"`
}

type recipeAutoreplyConfig struct {
	ExtraGuidance string `toml:"extra_guidance"`
}

type recipeOverviewConfig struct {
	Enabled       *bool  `toml:"enabled"`
	ExtraGuidance string `toml:"extra_guidance"`
}

type recipeAutogenConfig struct {
	ExtraGuidance string `toml:"extra_guidance"`
}
