package cli

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

type repoURLBuilder func(repository string) string

func normalizeRepo(provider string, gitlabHostOverride string, input string) (string, string, repoURLBuilder, error) {
	value := strings.TrimSpace(input)
	if value == "" {
		return "", "", nil, nil
	}

	provider = normalizeVCSProvider(provider)
	switch provider {
	case vcsProviderGitLab:
		return normalizeGitLabRepo(value, gitlabHostOverride)
	default:
		return normalizeGitHubRepo(value)
	}
}

func normalizeGitHubRepo(value string) (string, string, repoURLBuilder, error) {
	repository, buildRepoURL, err := parseRepositorySlugGitHub(value)
	if err != nil {
		return "", "", nil, fmt.Errorf("invalid --repo value %q", value)
	}
	repoURL := buildRepoURL(repository)
	return repository, repoURL, buildRepoURL, nil
}

func normalizeGitLabRepo(value string, gitlabHostOverride string) (string, string, repoURLBuilder, error) {
	repository, buildRepoURL, err := parseRepositorySlugGitLab(value, gitlabHostOverride)
	if err != nil {
		return "", "", nil, fmt.Errorf("invalid --repo value %q", value)
	}
	repoURL := ""
	if buildRepoURL != nil {
		repoURL = buildRepoURL(repository)
	}
	return repository, repoURL, buildRepoURL, nil
}

func parseRepositorySlugGitHub(value string) (string, repoURLBuilder, error) {
	if strings.Contains(value, "://") {
		repository, buildRepoURL, err := parseRepositoryFromURLGitHub(value)
		if err != nil {
			return "", nil, err
		}
		return repository, buildRepoURL, nil
	}

	if strings.Contains(value, "@") && strings.Contains(value, ":") {
		repository, buildRepoURL, err := parseRepositoryFromSSHGithub(value)
		if err != nil {
			return "", nil, err
		}
		return repository, buildRepoURL, nil
	}

	repository, err := parseRepositoryPathGitHub(value)
	if err != nil {
		return "", nil, err
	}
	return repository, httpsRepoURLBuilder, nil
}

func parseRepositorySlugGitLab(value string, gitlabHostOverride string) (string, repoURLBuilder, error) {
	if strings.Contains(value, "://") {
		repository, buildRepoURL, err := parseRepositoryFromURLGitLab(value)
		if err != nil {
			return "", nil, err
		}
		return repository, buildRepoURL, nil
	}

	if strings.Contains(value, "@") && strings.Contains(value, ":") {
		repository, buildRepoURL, err := parseRepositoryFromSSHGitLab(value)
		if err != nil {
			return "", nil, err
		}
		return repository, buildRepoURL, nil
	}

	repository, err := parseRepositoryPathGitLab(value)
	if err != nil {
		return "", nil, err
	}
	host := gitlabHostForRepo(gitlabHostOverride)
	return repository, gitlabHTTPSRepoURLBuilder(host), nil
}

func parseRepositoryFromURLGitHub(value string) (string, repoURLBuilder, error) {
	parsed, err := url.Parse(value)
	if err != nil {
		return "", nil, err
	}
	if parsed.Host != "github.com" {
		return "", nil, fmt.Errorf("unsupported repository host")
	}
	switch parsed.Scheme {
	case "http", "https", "ssh":
	default:
		return "", nil, fmt.Errorf("unsupported repository scheme")
	}

	repository, err := parseRepositoryPathGitHub(parsed.Path)
	if err != nil {
		return "", nil, err
	}
	switch parsed.Scheme {
	case "http":
		return repository, httpRepoURLBuilder, nil
	case "https":
		return repository, httpsRepoURLBuilder, nil
	case "ssh":
		if parsed.User != nil && parsed.User.Username() == "git" {
			return repository, sshRepoURLBuilder, nil
		}
		return "", nil, fmt.Errorf("unsupported ssh repository user")
	default:
		return "", nil, fmt.Errorf("unsupported repository scheme")
	}
}

func parseRepositoryFromURLGitLab(value string) (string, repoURLBuilder, error) {
	parsed, err := url.Parse(value)
	if err != nil {
		return "", nil, err
	}
	switch parsed.Scheme {
	case "http", "https", "ssh":
	default:
		return "", nil, fmt.Errorf("unsupported repository scheme")
	}

	path := strings.TrimSpace(parsed.Path)
	if strings.Contains(path, "/-/") {
		parts := strings.SplitN(path, "/-/", 2)
		if len(parts) > 0 {
			path = parts[0]
		}
	}
	repository, err := parseRepositoryPathGitLab(path)
	if err != nil {
		return "", nil, err
	}
	host := strings.TrimSpace(parsed.Host)
	if host == "" {
		return "", nil, fmt.Errorf("unsupported repository host")
	}
	switch parsed.Scheme {
	case "http":
		return repository, gitlabHTTPRepoURLBuilder(host), nil
	case "https":
		return repository, gitlabHTTPSRepoURLBuilder(host), nil
	case "ssh":
		if parsed.User != nil && parsed.User.Username() == "git" {
			return repository, gitlabSSHRepoURLBuilder(host), nil
		}
		return "", nil, fmt.Errorf("unsupported ssh repository user")
	default:
		return "", nil, fmt.Errorf("unsupported repository scheme")
	}
}

func parseRepositoryFromSSHGithub(value string) (string, repoURLBuilder, error) {
	parts := strings.SplitN(value, ":", 2)
	if len(parts) != 2 {
		return "", nil, fmt.Errorf("invalid ssh repository")
	}

	hostParts := strings.SplitN(parts[0], "@", 2)
	if len(hostParts) != 2 || hostParts[1] != "github.com" {
		return "", nil, fmt.Errorf("unsupported repository host")
	}
	if hostParts[0] != "git" {
		return "", nil, fmt.Errorf("unsupported ssh repository user")
	}

	repository, err := parseRepositoryPathGitHub(parts[1])
	if err != nil {
		return "", nil, err
	}
	return repository, gitAtRepoURLBuilder, nil
}

func parseRepositoryFromSSHGitLab(value string) (string, repoURLBuilder, error) {
	parts := strings.SplitN(value, ":", 2)
	if len(parts) != 2 {
		return "", nil, fmt.Errorf("invalid ssh repository")
	}

	hostParts := strings.SplitN(parts[0], "@", 2)
	if len(hostParts) != 2 {
		return "", nil, fmt.Errorf("unsupported repository host")
	}
	if hostParts[0] != "git" {
		return "", nil, fmt.Errorf("unsupported ssh repository user")
	}
	host := strings.TrimSpace(hostParts[1])
	if host == "" {
		return "", nil, fmt.Errorf("unsupported repository host")
	}

	repository, err := parseRepositoryPathGitLab(parts[1])
	if err != nil {
		return "", nil, err
	}
	return repository, gitlabGitAtRepoURLBuilder(host), nil
}

func parseRepositoryPathGitHub(path string) (string, error) {
	trimmed := strings.Trim(strings.TrimSuffix(path, ".git"), "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) != 2 {
		return "", fmt.Errorf("repository must use owner/repo format")
	}
	if strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return "", fmt.Errorf("repository owner and name are required")
	}
	return parts[0] + "/" + parts[1], nil
}

func parseRepositoryPathGitLab(path string) (string, error) {
	trimmed := strings.Trim(strings.TrimSuffix(path, ".git"), "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) < 2 {
		return "", fmt.Errorf("repository path must use group/project format")
	}
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			return "", fmt.Errorf("repository group and name are required")
		}
	}
	return strings.Join(parts, "/"), nil
}

func httpRepoURLBuilder(repository string) string {
	return fmt.Sprintf("http://github.com/%s.git", strings.TrimSpace(repository))
}

func httpsRepoURLBuilder(repository string) string {
	return fmt.Sprintf("https://github.com/%s.git", strings.TrimSpace(repository))
}

func sshRepoURLBuilder(repository string) string {
	return fmt.Sprintf("ssh://git@github.com/%s.git", strings.TrimSpace(repository))
}

func gitAtRepoURLBuilder(repository string) string {
	return fmt.Sprintf("git@github.com:%s.git", strings.TrimSpace(repository))
}

func gitlabHTTPRepoURLBuilder(host string) repoURLBuilder {
	host = strings.TrimSpace(host)
	return func(repository string) string {
		return fmt.Sprintf("http://%s/%s.git", host, strings.TrimSpace(repository))
	}
}

func gitlabHTTPSRepoURLBuilder(host string) repoURLBuilder {
	host = strings.TrimSpace(host)
	return func(repository string) string {
		return fmt.Sprintf("https://%s/%s.git", host, strings.TrimSpace(repository))
	}
}

func gitlabSSHRepoURLBuilder(host string) repoURLBuilder {
	host = strings.TrimSpace(host)
	return func(repository string) string {
		return fmt.Sprintf("ssh://git@%s/%s.git", host, strings.TrimSpace(repository))
	}
}

func gitlabGitAtRepoURLBuilder(host string) repoURLBuilder {
	host = strings.TrimSpace(host)
	return func(repository string) string {
		return fmt.Sprintf("git@%s:%s.git", host, strings.TrimSpace(repository))
	}
}

func gitlabHostForRepo(override string) string {
	host := strings.TrimSpace(override)
	if host == "" {
		host = strings.TrimSpace(os.Getenv("GITLAB_HOST"))
	}
	if host == "" {
		host = "gitlab.com"
	}
	return host
}
