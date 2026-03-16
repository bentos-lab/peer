package cli

import (
	"fmt"
	"strings"
)

// ParseVCSProvider parses provider flags like "gitlab:host" into provider and host.
func ParseVCSProvider(raw string) (string, string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "github", "", nil
	}

	parts := strings.SplitN(trimmed, ":", 2)
	provider := strings.ToLower(strings.TrimSpace(parts[0]))
	if provider == "" {
		return "", "", fmt.Errorf("invalid vcs provider value")
	}

	host := ""
	if len(parts) == 2 {
		host = strings.TrimSpace(parts[1])
		if host == "" {
			return "", "", fmt.Errorf("invalid vcs provider host")
		}
		if provider != "gitlab" {
			return "", "", fmt.Errorf("invalid vcs provider host")
		}
	}

	return provider, host, nil
}
