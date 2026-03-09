package github

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"bentos-backend/adapter/outbound/commandrunner"
	"bentos-backend/domain"
)

// CLIClient executes GitHub operations via the gh CLI.
type CLIClient struct {
	runner commandrunner.Runner
}

// CreateReviewCommentInput contains one anchored GitHub review comment payload.
type CreateReviewCommentInput struct {
	Body      string
	Path      string
	StartLine int
	EndLine   int
}

// PullRequestInfo contains normalized pull-request metadata.
type PullRequestInfo struct {
	Repository  string
	Number      int
	Title       string
	Description string
	BaseRef     string
	HeadRef     string
}

// NewCLIClient creates a GitHub CLI API client.
func NewCLIClient() *CLIClient {
	return &CLIClient{
		runner: commandrunner.NewOSCommandRunner(),
	}
}

// GetPullRequestChangedFiles loads changed files for a pull request.
func (c *CLIClient) GetPullRequestChangedFiles(ctx context.Context, repository string, pullRequestNumber int) ([]domain.ChangedFile, error) {
	if pullRequestNumber <= 0 {
		return nil, fmt.Errorf("pull request number must be positive")
	}
	if err := c.ensureAuth(ctx); err != nil {
		return nil, err
	}

	resolvedRepo, err := c.ResolveRepository(ctx, repository)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("repos/%s/pulls/%d/files", resolvedRepo, pullRequestNumber)
	commandResult, err := c.runner.Run(ctx, "gh", "api", endpoint, "--paginate")
	if err != nil {
		return nil, fmt.Errorf("failed to load pull request changed files: %w", formatCommandError(err, commandResult))
	}

	files, err := parsePullRequestFiles(commandResult.Stdout)
	if err != nil {
		return nil, fmt.Errorf("failed to parse pull request files response: %w", err)
	}

	return mapPullRequestFilesToChangedFiles(files), nil
}

// CreateComment posts a comment to GitHub.
func (c *CLIClient) CreateComment(ctx context.Context, repository string, pullRequestNumber int, body string) error {
	if pullRequestNumber <= 0 {
		return fmt.Errorf("pull request number must be positive")
	}
	if err := c.ensureAuth(ctx); err != nil {
		return err
	}

	resolvedRepo, err := c.ResolveRepository(ctx, repository)
	if err != nil {
		return err
	}

	result, err := c.runner.Run(
		ctx,
		"gh",
		"pr",
		"comment",
		strconv.Itoa(pullRequestNumber),
		"--repo",
		resolvedRepo,
		"--body",
		body,
	)
	if err != nil {
		return fmt.Errorf("failed to create pull request comment: %w", formatCommandError(err, result))
	}
	return nil
}

// CreateReviewComment posts a file-anchored review comment to GitHub.
func (c *CLIClient) CreateReviewComment(ctx context.Context, repository string, pullRequestNumber int, input CreateReviewCommentInput) error {
	if pullRequestNumber <= 0 {
		return fmt.Errorf("pull request number must be positive")
	}
	if err := c.ensureAuth(ctx); err != nil {
		return err
	}

	resolvedRepo, err := c.ResolveRepository(ctx, repository)
	if err != nil {
		return err
	}

	headSHA, err := c.getPullRequestHeadSHA(ctx, resolvedRepo, pullRequestNumber)
	if err != nil {
		return err
	}

	args := []string{
		"api",
		"--method",
		"POST",
		fmt.Sprintf("repos/%s/pulls/%d/comments", resolvedRepo, pullRequestNumber),
		"--raw-field",
		"body=" + input.Body,
		"--raw-field",
		"path=" + input.Path,
		"--raw-field",
		"side=RIGHT",
		"--raw-field",
		"commit_id=" + headSHA,
		"--field",
		"line=" + strconv.Itoa(input.EndLine),
	}
	if input.StartLine != input.EndLine {
		args = append(
			args,
			"--field",
			"start_line="+strconv.Itoa(input.StartLine),
			"--raw-field",
			"start_side=RIGHT",
		)
	}

	result, err := c.runner.Run(ctx, "gh", args...)
	if err != nil {
		cause := formatCommandError(err, result)
		if isInvalidAnchorCommandError(cause) {
			return &InvalidAnchorError{
				Message: "invalid review comment anchor",
				Cause:   cause,
			}
		}
		return fmt.Errorf("failed to create pull request review comment: %w", cause)
	}

	return nil
}

// ResolveRepository resolves the effective GitHub repository slug.
func (c *CLIClient) ResolveRepository(ctx context.Context, repository string) (string, error) {
	return c.resolveRepository(ctx, repository)
}

// GetPullRequestInfo loads title/body/base/head for a pull request.
func (c *CLIClient) GetPullRequestInfo(ctx context.Context, repository string, pullRequestNumber int) (PullRequestInfo, error) {
	if pullRequestNumber <= 0 {
		return PullRequestInfo{}, fmt.Errorf("pull request number must be positive")
	}
	if err := c.ensureAuth(ctx); err != nil {
		return PullRequestInfo{}, err
	}
	resolvedRepo, err := c.ResolveRepository(ctx, repository)
	if err != nil {
		return PullRequestInfo{}, err
	}

	result, err := c.runner.Run(
		ctx,
		"gh",
		"api",
		fmt.Sprintf("repos/%s/pulls/%d", resolvedRepo, pullRequestNumber),
	)
	if err != nil {
		return PullRequestInfo{}, fmt.Errorf("failed to load pull request metadata: %w", formatCommandError(err, result))
	}

	var payload struct {
		Title string `json:"title"`
		Body  string `json:"body"`
		Base  struct {
			Ref string `json:"ref"`
			SHA string `json:"sha"`
		} `json:"base"`
		Head struct {
			Ref string `json:"ref"`
			SHA string `json:"sha"`
		} `json:"head"`
	}
	if err := json.Unmarshal(result.Stdout, &payload); err != nil {
		return PullRequestInfo{}, fmt.Errorf("failed to parse pull request metadata: %w", err)
	}

	base := strings.TrimSpace(payload.Base.SHA)
	if base == "" {
		base = strings.TrimSpace(payload.Base.Ref)
	}
	head := strings.TrimSpace(payload.Head.SHA)
	if head == "" {
		head = strings.TrimSpace(payload.Head.Ref)
	}
	if base == "" || head == "" {
		return PullRequestInfo{}, fmt.Errorf("failed to resolve pull request base/head refs")
	}

	return PullRequestInfo{
		Repository:  resolvedRepo,
		Number:      pullRequestNumber,
		Title:       strings.TrimSpace(payload.Title),
		Description: strings.TrimSpace(payload.Body),
		BaseRef:     base,
		HeadRef:     head,
	}, nil
}

func (c *CLIClient) getPullRequestHeadSHA(ctx context.Context, repository string, pullRequestNumber int) (string, error) {
	result, err := c.runner.Run(
		ctx,
		"gh",
		"api",
		fmt.Sprintf("repos/%s/pulls/%d", repository, pullRequestNumber),
	)
	if err != nil {
		return "", fmt.Errorf("failed to load pull request metadata: %w", formatCommandError(err, result))
	}

	var payload struct {
		Head struct {
			SHA string `json:"sha"`
		} `json:"head"`
	}
	if err := json.Unmarshal(result.Stdout, &payload); err != nil {
		return "", fmt.Errorf("failed to parse pull request metadata: %w", err)
	}
	payload.Head.SHA = strings.TrimSpace(payload.Head.SHA)
	if payload.Head.SHA == "" {
		return "", fmt.Errorf("failed to resolve pull request head commit")
	}
	return payload.Head.SHA, nil
}

func (c *CLIClient) ensureAuth(ctx context.Context) error {
	result, err := c.runner.Run(ctx, "gh", "auth", "status")
	if err != nil {
		return fmt.Errorf("github CLI is not authenticated; run `gh auth login` first: %w", formatCommandError(err, result))
	}
	return nil
}

func (c *CLIClient) resolveRepository(ctx context.Context, repository string) (string, error) {
	repository = strings.TrimSpace(repository)
	if repository != "" {
		return repository, nil
	}

	result, err := c.runner.Run(ctx, "gh", "repo", "view", "--json", "nameWithOwner")
	if err != nil {
		return "", fmt.Errorf("failed to resolve current GitHub repository: %w", formatCommandError(err, result))
	}

	var payload struct {
		NameWithOwner string `json:"nameWithOwner"`
	}
	if err := json.Unmarshal(result.Stdout, &payload); err != nil {
		return "", err
	}
	payload.NameWithOwner = strings.TrimSpace(payload.NameWithOwner)
	if payload.NameWithOwner == "" {
		return "", fmt.Errorf("failed to resolve current GitHub repository")
	}
	return payload.NameWithOwner, nil
}

func formatCommandError(err error, result commandrunner.Result) error {
	message := strings.TrimSpace(string(result.Stderr))
	if message == "" {
		message = strings.TrimSpace(string(result.Stdout))
	}
	if message == "" {
		return err
	}
	return fmt.Errorf("%w: %s", err, message)
}
