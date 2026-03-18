package customrecipe

import (
	"os"
	"strconv"
	"strings"

	"github.com/bentos-lab/peer/usecase"
)

const (
	envReviewEnabled                 = "REVIEW"
	envReviewSuggestions             = "REVIEW_SUGGESTED_CHANGES"
	envReviewEvents                  = "REVIEW_EVENTS"
	envOverviewEnabled               = "OVERVIEW"
	envOverviewEvents                = "OVERVIEW_EVENTS"
	envOverviewIssueAlignmentEnabled = "OVERVIEW_ISSUE_ALIGNMENT"
	envReplyCommentEnabled           = "REPLYCOMMENT"
	envReplyCommentEvents            = "REPLYCOMMENT_EVENTS"
	envReplyCommentActions           = "REPLYCOMMENT_ACTIONS"
	envAutogenEnabled                = "AUTOGEN"
	envAutogenEvents                 = "AUTOGEN_EVENTS"
	envAutogenDocs                   = "AUTOGEN_DOCS"
	envAutogenTests                  = "AUTOGEN_TESTS"
)

type optionalStringList struct {
	Values []string
	Set    bool
}

type recipeEnvDefaults struct {
	ReviewEnabled                 *bool
	ReviewSuggestions             *bool
	ReviewEvents                  optionalStringList
	OverviewEnabled               *bool
	OverviewEvents                optionalStringList
	OverviewIssueAlignmentEnabled *bool
	ReplyCommentEnabled           *bool
	ReplyCommentEvents            optionalStringList
	ReplyCommentActions           optionalStringList
	AutogenEnabled                *bool
	AutogenEvents                 optionalStringList
	AutogenDocs                   *bool
	AutogenTests                  *bool
}

func loadRecipeEnvDefaults(logger usecase.Logger) recipeEnvDefaults {
	return recipeEnvDefaults{
		ReviewEnabled:                 envOptionalBool(logger, envReviewEnabled),
		ReviewSuggestions:             envOptionalBool(logger, envReviewSuggestions),
		ReviewEvents:                  envOptionalStringList(envReviewEvents),
		OverviewEnabled:               envOptionalBool(logger, envOverviewEnabled),
		OverviewEvents:                envOptionalStringList(envOverviewEvents),
		OverviewIssueAlignmentEnabled: envOptionalBool(logger, envOverviewIssueAlignmentEnabled),
		ReplyCommentEnabled:           envOptionalBool(logger, envReplyCommentEnabled),
		ReplyCommentEvents:            envOptionalStringList(envReplyCommentEvents),
		ReplyCommentActions:           envOptionalStringList(envReplyCommentActions),
		AutogenEnabled:                envOptionalBool(logger, envAutogenEnabled),
		AutogenEvents:                 envOptionalStringList(envAutogenEvents),
		AutogenDocs:                   envOptionalBool(logger, envAutogenDocs),
		AutogenTests:                  envOptionalBool(logger, envAutogenTests),
	}
}

func envOptionalBool(logger usecase.Logger, key string) *bool {
	rawValue, exists := os.LookupEnv(key)
	if !exists {
		return nil
	}
	parsedValue, err := strconv.ParseBool(strings.TrimSpace(rawValue))
	if err != nil {
		if logger != nil {
			logger.Warnf("Custom recipe env %q is invalid: %v", key, err)
		}
		return nil
	}
	return &parsedValue
}

func envOptionalStringList(key string) optionalStringList {
	rawValue, exists := os.LookupEnv(key)
	if !exists {
		return optionalStringList{}
	}
	trimmed := strings.TrimSpace(rawValue)
	if trimmed == "" {
		return optionalStringList{Values: []string{}, Set: true}
	}
	parts := strings.Split(trimmed, ",")
	normalized := normalizeStringList(parts)
	if normalized == nil {
		normalized = []string{}
	}
	return optionalStringList{Values: normalized, Set: true}
}

func normalizeStringList(values []string) []string {
	if values == nil {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		lowered := strings.ToLower(trimmed)
		if _, exists := seen[lowered]; exists {
			continue
		}
		seen[lowered] = struct{}{}
		normalized = append(normalized, lowered)
	}
	if normalized == nil {
		return []string{}
	}
	return normalized
}
