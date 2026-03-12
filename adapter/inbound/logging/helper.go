package logging

import (
	"net/url"
	"sort"
	"strings"
)

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
