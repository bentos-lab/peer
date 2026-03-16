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

	recipe, err := l.loadEnvRecipeDefaults()
	if err != nil {
		return domain.CustomRecipe{}, err
	}
	recipe, err = OverrideFromCodeEnv(ctx, env, headRef, recipe, l.logger)
	if err != nil {
		return domain.CustomRecipe{}, err
	}

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

	if parsed.Review.Ruleset != nil {
		reviewRuleset, missingPath, err := l.readAndSanitize(ctx, env, headRef, *parsed.Review.Ruleset, l.readOnlySanitizer)
		if err != nil {
			return domain.CustomRecipe{}, err
		}
		if missingPath != "" {
			recipe.MissingPaths = append(recipe.MissingPaths, missingPath)
		}
		recipe.ReviewRuleset = reviewRuleset
	}

	if parsed.Overview.ExtraGuidance != nil {
		overviewGuidance, missingPath, err := l.readAndSanitize(ctx, env, headRef, *parsed.Overview.ExtraGuidance, l.readOnlySanitizer)
		if err != nil {
			return domain.CustomRecipe{}, err
		}
		if missingPath != "" {
			recipe.MissingPaths = append(recipe.MissingPaths, missingPath)
		}
		recipe.OverviewGuidance = overviewGuidance
	}

	if parsed.Overview.IssueAlignment.ExtraGuidance != nil {
		issueAlignmentGuidance, missingPath, err := l.readAndSanitize(ctx, env, headRef, *parsed.Overview.IssueAlignment.ExtraGuidance, l.readOnlySanitizer)
		if err != nil {
			return domain.CustomRecipe{}, err
		}
		if missingPath != "" {
			recipe.MissingPaths = append(recipe.MissingPaths, missingPath)
		}
		recipe.OverviewIssueAlignmentGuidance = issueAlignmentGuidance
	}

	if parsed.ReplyComment.ExtraGuidance != nil {
		replyCommentGuidance, missingPath, err := l.readAndSanitize(ctx, env, headRef, *parsed.ReplyComment.ExtraGuidance, l.readOnlySanitizer)
		if err != nil {
			return domain.CustomRecipe{}, err
		}
		if missingPath != "" {
			recipe.MissingPaths = append(recipe.MissingPaths, missingPath)
		}
		recipe.ReplyCommentGuidance = replyCommentGuidance
	}

	if parsed.Autogen.ExtraGuidance != nil {
		autogenGuidance, missingPath, err := l.readAndSanitize(ctx, env, headRef, *parsed.Autogen.ExtraGuidance, l.readWriteSanitizer)
		if err != nil {
			return domain.CustomRecipe{}, err
		}
		if missingPath != "" {
			recipe.MissingPaths = append(recipe.MissingPaths, missingPath)
		}
		recipe.AutogenGuidance = autogenGuidance
	}

	return recipe, nil
}

func (l *Loader) loadEnvRecipeDefaults() (domain.CustomRecipe, error) {
	return LoadDefaultsFromEnv(l.logger), nil
}
