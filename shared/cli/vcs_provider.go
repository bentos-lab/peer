package cli

import (
	"fmt"
	"net/url"
	"strings"
)

const (
	vcsProviderGitHub = "github"
	vcsProviderGitLab = "gitlab"
)

var supportedVCSProviders = []string{
	vcsProviderGitHub,
	vcsProviderGitLab,
	vcsProviderGitLab + ":host",
}

// SupportedVCSProviderValuesText returns supported values for --vcs-provider.
func SupportedVCSProviderValuesText() string {
	return strings.Join(supportedVCSProviders, ", ")
}

// VCSProviderFlagHelp returns the help text for --vcs-provider flags.
func VCSProviderFlagHelp() string {
	return fmt.Sprintf("vcs provider name (%s)", SupportedVCSProviderValuesText())
}

// VCSProviderListFlagHelp returns the help text for --vcs-provider list flags.
func VCSProviderListFlagHelp() string {
	return fmt.Sprintf("vcs provider names joined by + (supported: %s)", SupportedVCSProviderValuesText())
}

// ParseVCSProvider parses provider flags like "gitlab:host" into provider and host.
func ParseVCSProvider(raw string) (string, string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", "", fmt.Errorf("invalid vcs provider value (supported: %s)", SupportedVCSProviderValuesText())
	}

	parts := strings.SplitN(trimmed, ":", 2)
	provider := strings.ToLower(strings.TrimSpace(parts[0]))
	if provider == "" {
		return "", "", fmt.Errorf("invalid vcs provider value (supported: %s)", SupportedVCSProviderValuesText())
	}

	host := ""
	if len(parts) == 2 {
		host = strings.TrimSpace(parts[1])
		if host == "" {
			return "", "", fmt.Errorf("invalid vcs provider host (supported: %s)", SupportedVCSProviderValuesText())
		}
		if provider != "gitlab" {
			return "", "", fmt.Errorf("invalid vcs provider host (supported: %s)", SupportedVCSProviderValuesText())
		}
	}

	switch provider {
	case vcsProviderGitHub, vcsProviderGitLab:
	default:
		return "", "", fmt.Errorf("unsupported vcs provider: %s (supported: %s)", provider, SupportedVCSProviderValuesText())
	}
	return provider, host, nil
}

// ResolveVCSProviderFromRepo detects provider from a repository URL or SSH remote.
func ResolveVCSProviderFromRepo(repo string) (string, string, error) {
	trimmed := strings.TrimSpace(repo)
	if trimmed == "" {
		return "", "", fmt.Errorf("repository is required")
	}

	var host string
	if strings.Contains(trimmed, "://") {
		parsed, err := url.Parse(trimmed)
		if err != nil {
			return "", "", err
		}
		host = strings.TrimSpace(parsed.Host)
	} else if strings.Contains(trimmed, "@") && strings.Contains(trimmed, ":") {
		parts := strings.SplitN(trimmed, ":", 2)
		if len(parts) == 2 {
			hostParts := strings.SplitN(parts[0], "@", 2)
			if len(hostParts) == 2 {
				host = strings.TrimSpace(hostParts[1])
			}
		}
	} else {
		return "", "", fmt.Errorf("ambiguous repository reference")
	}

	if host == "" {
		return "", "", fmt.Errorf("repository host is required")
	}
	lowerHost := strings.ToLower(host)
	switch {
	case strings.Contains(lowerHost, "github"):
		return vcsProviderGitHub, "", nil
	case strings.Contains(lowerHost, "gitlab"):
		return vcsProviderGitLab, host, nil
	default:
		return "", "", fmt.Errorf("unsupported repository host")
	}
}
