package customrecipe

import (
	"strings"

	"bentos-backend/domain"
	"bentos-backend/usecase"

	"github.com/pelletier/go-toml/v2"
)

type recipeConfig struct {
	Review       recipeReviewConfig       `toml:"review"`
	Overview     recipeOverviewConfig     `toml:"overview"`
	ReplyComment recipeReplyCommentConfig `toml:"replycomment"`
	Autogen      recipeAutogenConfig      `toml:"autogen"`
}

type recipeReviewConfig struct {
	Enabled     *bool    `toml:"enabled"`
	Ruleset     *string  `toml:"ruleset"`
	Suggestions *bool    `toml:"suggestions"`
	Events      []string `toml:"events"`
}

type recipeReplyCommentConfig struct {
	Enabled       *bool    `toml:"enabled"`
	ExtraGuidance *string  `toml:"extra_guidance"`
	Events        []string `toml:"events"`
	Actions       []string `toml:"actions"`
}

type recipeOverviewConfig struct {
	Enabled        *bool                      `toml:"enabled"`
	ExtraGuidance  *string                    `toml:"extra_guidance"`
	Events         []string                   `toml:"events"`
	IssueAlignment recipeIssueAlignmentConfig `toml:"issue_alignment"`
}

type recipeIssueAlignmentConfig struct {
	Enabled       *bool   `toml:"enabled"`
	ExtraGuidance *string `toml:"extra_guidance"`
}

type recipeAutogenConfig struct {
	Enabled       *bool    `toml:"enabled"`
	ExtraGuidance *string  `toml:"extra_guidance"`
	Events        []string `toml:"events"`
	Docs          *bool    `toml:"docs"`
	Tests         *bool    `toml:"tests"`
}

func parseRecipeConfig(rawConfig string, logger usecase.Logger) (recipeConfig, bool) {
	if strings.TrimSpace(rawConfig) == "" {
		return recipeConfig{}, false
	}

	var parsed recipeConfig
	if err := toml.Unmarshal([]byte(rawConfig), &parsed); err != nil {
		if logger != nil {
			logger.Warnf("Custom recipe config is invalid: %v", err)
		}
		return recipeConfig{}, false
	}
	return parsed, true
}

func applyRecipeOverrides(recipe domain.CustomRecipe, parsed recipeConfig) domain.CustomRecipe {
	if parsed.Review.Enabled != nil {
		recipe.ReviewEnabled = parsed.Review.Enabled
	}
	if parsed.Review.Suggestions != nil {
		recipe.ReviewSuggestions = parsed.Review.Suggestions
	}
	if parsed.Review.Events != nil {
		recipe.ReviewEvents = normalizeStringList(parsed.Review.Events)
	}
	if parsed.Overview.Enabled != nil {
		recipe.OverviewEnabled = parsed.Overview.Enabled
	}
	if parsed.Overview.Events != nil {
		recipe.OverviewEvents = normalizeStringList(parsed.Overview.Events)
	}
	if parsed.Overview.IssueAlignment.Enabled != nil {
		recipe.OverviewIssueAlignmentEnabled = parsed.Overview.IssueAlignment.Enabled
	}
	if parsed.ReplyComment.Enabled != nil {
		recipe.ReplyCommentEnabled = parsed.ReplyComment.Enabled
	}
	if parsed.ReplyComment.Events != nil {
		recipe.ReplyCommentEvents = normalizeStringList(parsed.ReplyComment.Events)
	}
	if parsed.ReplyComment.Actions != nil {
		recipe.ReplyCommentActions = normalizeStringList(parsed.ReplyComment.Actions)
	}
	if parsed.Autogen.Enabled != nil {
		recipe.AutogenEnabled = parsed.Autogen.Enabled
	}
	if parsed.Autogen.Events != nil {
		recipe.AutogenEvents = normalizeStringList(parsed.Autogen.Events)
	}
	if parsed.Autogen.Docs != nil {
		recipe.AutogenDocs = parsed.Autogen.Docs
	}
	if parsed.Autogen.Tests != nil {
		recipe.AutogenTests = parsed.Autogen.Tests
	}
	return recipe
}
