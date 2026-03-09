package logging

import (
	"net/url"
	"sort"
	"strings"

	"bentos-backend/shared/logger/stdlogger"
	"bentos-backend/usecase"
)

// LogChangeRequestInputSnapshot logs a safe summary of inbound parameters before usecase execution.
func LogChangeRequestInputSnapshot(logger usecase.Logger, source string, action string, request usecase.ChangeRequestRequest) {
	if logger == nil {
		logger = stdlogger.Nop()
	}

	trimmedAction := strings.TrimSpace(action)
	if trimmedAction == "" {
		logger.Infof(
			"Pre-usecase input snapshot source=%q provider=%q repository=%q changeRequestNumber=%d enableOverview=%t enableSuggestions=%t.",
			strings.TrimSpace(source),
			request.Provider,
			request.Repository,
			request.ChangeRequestNumber,
			request.EnableOverview,
			request.EnableSuggestions,
		)
	} else {
		logger.Infof(
			"Pre-usecase input snapshot source=%q action=%q provider=%q repository=%q changeRequestNumber=%d enableOverview=%t enableSuggestions=%t.",
			strings.TrimSpace(source),
			trimmedAction,
			request.Provider,
			request.Repository,
			request.ChangeRequestNumber,
			request.EnableOverview,
			request.EnableSuggestions,
		)
	}

	safeRepoURL, hasRepoURL := sanitizeRepoURL(request.RepoURL)
	logger.Debugf(
		"Pre-usecase input details source=%q action=%q base=%q head=%q comment=%t metadataKeys=%q metadataCount=%d titleLength=%d descriptionLength=%d repoURLPresent=%t repoURLSafe=%q.",
		strings.TrimSpace(source),
		trimmedAction,
		request.Base,
		request.Head,
		request.Comment,
		strings.Join(sortedMetadataKeys(request.Metadata), ","),
		len(request.Metadata),
		len(request.Title),
		len(request.Description),
		hasRepoURL,
		safeRepoURL,
	)
}

func sortedMetadataKeys(metadata map[string]string) []string {
	if len(metadata) == 0 {
		return nil
	}

	keys := make([]string, 0, len(metadata))
	for key := range metadata {
		trimmed := strings.TrimSpace(key)
		if trimmed == "" {
			continue
		}
		keys = append(keys, trimmed)
	}
	sort.Strings(keys)
	return keys
}

func sanitizeRepoURL(rawRepoURL string) (string, bool) {
	trimmed := strings.TrimSpace(rawRepoURL)
	if trimmed == "" {
		return "", false
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "present", true
	}

	if parsed.Scheme == "" || parsed.Host == "" {
		return "present", true
	}

	parsed.User = nil
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), true
}
