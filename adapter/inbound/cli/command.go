package cli

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	inboundlogging "bentos-backend/adapter/inbound/logging"
	githubvcs "bentos-backend/adapter/outbound/vcs/github"
	"bentos-backend/shared/logger/stdlogger"
	"bentos-backend/usecase"
)

// GitHubClient resolves repository and pull-request metadata.
type GitHubClient interface {
	ResolveRepository(ctx context.Context, repository string) (string, error)
	GetPullRequestInfo(ctx context.Context, repository string, pullRequestNumber int) (githubvcs.PullRequestInfo, error)
}

// Command runs autogit review flow with the shared change request usecase.
type Command struct {
	changeRequestUseCase usecase.ChangeRequestUseCase
	githubClient         GitHubClient
	logger               usecase.Logger
}

// RunParams contains already-parsed CLI autogit parameters.
type RunParams struct {
	Provider      string
	Repo          string
	ChangeRequest string
	Base          string
	Head          string
	Comment       bool
	Overview      bool
	Suggest       bool
}

type repoURLBuilder func(repository string) string

// NewCommand creates a new CLI command for autogit reviews.
func NewCommand(changeRequestUseCase usecase.ChangeRequestUseCase, githubClient GitHubClient, logger usecase.Logger) *Command {
	if logger == nil {
		logger = stdlogger.Nop()
	}
	return &Command{
		changeRequestUseCase: changeRequestUseCase,
		githubClient:         githubClient,
		logger:               logger,
	}
}

// Run executes the CLI review flow.
func (c *Command) Run(ctx context.Context, params RunParams) error {
	if c.changeRequestUseCase == nil {
		return errors.New("change request usecase is not configured")
	}
	if c.githubClient == nil {
		return errors.New("github client is not configured")
	}
	if c.logger == nil {
		c.logger = stdlogger.Nop()
	}

	provider := strings.TrimSpace(strings.ToLower(params.Provider))
	if provider == "" {
		provider = "github"
	}
	if provider != "github" {
		return fmt.Errorf("unsupported provider: %s", provider)
	}

	if strings.TrimSpace(params.ChangeRequest) != "" && (strings.TrimSpace(params.Base) != "" || strings.TrimSpace(params.Head) != "") {
		return errors.New("--change-request cannot be used with --base or --head")
	}
	if params.Comment && strings.TrimSpace(params.ChangeRequest) == "" {
		return errors.New("--comment requires --change-request")
	}

	repository, repoURL, buildRepoURL, err := normalizeRepo(params.Repo)
	if err != nil {
		return err
	}
	repoProvided := strings.TrimSpace(params.Repo) != ""
	repository, err = c.githubClient.ResolveRepository(ctx, repository)
	if err != nil {
		return err
	}

	base := strings.TrimSpace(params.Base)
	head := strings.TrimSpace(params.Head)
	if head == "" {
		if repoProvided {
			head = "HEAD"
		} else {
			head = "@staged"
		}
	}
	if base == "" && head != "" {
		base = "HEAD"
	}

	prNumber := 0
	title := ""
	description := ""
	if strings.TrimSpace(params.ChangeRequest) != "" {
		parsed, parseErr := strconv.Atoi(strings.TrimSpace(params.ChangeRequest))
		if parseErr != nil || parsed <= 0 {
			return fmt.Errorf("--change-request must be a positive integer")
		}
		prNumber = parsed
		prInfo, infoErr := c.githubClient.GetPullRequestInfo(ctx, repository, prNumber)
		if infoErr != nil {
			return infoErr
		}
		repository = prInfo.Repository
		if repoProvided && buildRepoURL != nil {
			repoURL = buildRepoURL(prInfo.Repository)
		}
		base = prInfo.BaseRef
		head = prInfo.HeadRef
		title = prInfo.Title
		description = prInfo.Description
	}
	if repoProvided && isWorkspaceHeadToken(head) {
		return fmt.Errorf("--head %s requires local workspace mode; omit --repo", head)
	}

	request := usecase.ChangeRequestRequest{
		Provider:            provider,
		Repository:          repository,
		RepoURL:             repoURL,
		ChangeRequestNumber: prNumber,
		Title:               title,
		Description:         description,
		Base:                base,
		Head:                head,
		Comment:             params.Comment,
		EnableOverview:      params.Overview,
		EnableSuggestions:   params.Suggest,
		Metadata:            map[string]string{},
	}
	if !params.Comment {
		request.ChangeRequestNumber = 0
	}

	startedAt := time.Now()
	c.logger.Infof("CLI review started.")
	c.logger.Debugf("Provider is %q.", provider)
	c.logger.Debugf("Repository is %q and change request number is %d.", request.Repository, request.ChangeRequestNumber)
	inboundlogging.LogChangeRequestInputSnapshot(c.logger, "cli", "", request)

	_, err = c.changeRequestUseCase.Execute(ctx, request)
	if err != nil {
		c.logger.Errorf("CLI review failed.")
		c.logger.Debugf("The review used provider %q for repository %q and change request %d.", provider, request.Repository, request.ChangeRequestNumber)
		c.logger.Debugf("The CLI review ran for %d ms before failing.", time.Since(startedAt).Milliseconds())
		c.logger.Debugf("Failure details: %v.", err)
		return err
	}

	c.logger.Infof("CLI review completed.")
	c.logger.Debugf("The review used provider %q for repository %q and change request %d.", provider, request.Repository, request.ChangeRequestNumber)
	c.logger.Debugf("The CLI review completed in %d ms.", time.Since(startedAt).Milliseconds())
	return nil
}

func isWorkspaceHeadToken(head string) bool {
	switch strings.TrimSpace(head) {
	case "@staged", "@all":
		return true
	default:
		return false
	}
}

func normalizeRepo(input string) (string, string, repoURLBuilder, error) {
	value := strings.TrimSpace(input)
	if value == "" {
		return "", "", nil, nil
	}

	repository, buildRepoURL, err := parseRepositorySlug(value)
	if err != nil {
		return "", "", nil, fmt.Errorf("invalid --repo value %q", input)
	}
	repoURL := buildRepoURL(repository)
	return repository, repoURL, buildRepoURL, nil
}

func parseRepositorySlug(value string) (string, repoURLBuilder, error) {
	if strings.Contains(value, "://") {
		repository, buildRepoURL, err := parseRepositoryFromURL(value)
		if err != nil {
			return "", nil, err
		}
		return repository, buildRepoURL, nil
	}

	if strings.Contains(value, "@") && strings.Contains(value, ":") {
		repository, buildRepoURL, err := parseRepositoryFromSSH(value)
		if err != nil {
			return "", nil, err
		}
		return repository, buildRepoURL, nil
	}

	repository, err := parseRepositoryPath(value)
	if err != nil {
		return "", nil, err
	}
	return repository, httpsRepoURLBuilder, nil
}

func parseRepositoryFromURL(value string) (string, repoURLBuilder, error) {
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

	repository, err := parseRepositoryPath(parsed.Path)
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

func parseRepositoryFromSSH(value string) (string, repoURLBuilder, error) {
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

	repository, err := parseRepositoryPath(parts[1])
	if err != nil {
		return "", nil, err
	}
	return repository, gitAtRepoURLBuilder, nil
}

func parseRepositoryPath(path string) (string, error) {
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
