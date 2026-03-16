package cli

import "strings"

// resolveBaseHeadDefaults normalizes base/head with CLI defaults.
func resolveBaseHeadDefaults(base string, head string, repoProvided bool) (string, string) {
	normalizedBase := strings.TrimSpace(base)
	normalizedHead := strings.TrimSpace(head)

	if normalizedBase == "" && normalizedHead == "" {
		if repoProvided {
			return "HEAD", "HEAD"
		}
		return "HEAD", "@all"
	}

	if normalizedHead == "" {
		if repoProvided {
			normalizedHead = "HEAD"
		} else {
			normalizedHead = "@all"
		}
	}
	if normalizedBase == "" {
		normalizedBase = "HEAD"
	}

	return normalizedBase, normalizedHead
}
