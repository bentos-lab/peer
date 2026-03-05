package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

// InvalidAnchorError means GitHub rejected the requested file/line anchor.
type InvalidAnchorError struct {
	Message string
	Cause   error
}

// Error returns the invalid-anchor error message.
func (e *InvalidAnchorError) Error() string {
	if e.Cause == nil {
		return e.Message
	}
	return fmt.Sprintf("%s: %v", e.Message, e.Cause)
}

// Unwrap returns the underlying cause.
func (e *InvalidAnchorError) Unwrap() error {
	return e.Cause
}

// IsInvalidAnchorError reports whether err wraps InvalidAnchorError.
func IsInvalidAnchorError(err error) bool {
	var invalidAnchorErr *InvalidAnchorError
	return errors.As(err, &invalidAnchorErr)
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

	resolvedRepo, err := c.resolveRepository(ctx, repository)
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

	result := make([]domain.ChangedFile, 0)
	for _, item := range files {
		patch := strings.TrimSpace(item.Patch)
		if patch == "" {
			continue
		}
		result = append(result, domain.ChangedFile{
			Path:        item.Filename,
			Content:     patch,
			DiffSnippet: patch,
		})
	}

	return result, nil
}

// CreateComment posts a comment to GitHub.
func (c *CLIClient) CreateComment(ctx context.Context, repository string, pullRequestNumber int, body string) error {
	if pullRequestNumber <= 0 {
		return fmt.Errorf("pull request number must be positive")
	}
	if err := c.ensureAuth(ctx); err != nil {
		return err
	}

	resolvedRepo, err := c.resolveRepository(ctx, repository)
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

	resolvedRepo, err := c.resolveRepository(ctx, repository)
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

type pullRequestFile struct {
	Filename string `json:"filename"`
	Patch    string `json:"patch"`
}

func parsePullRequestFiles(raw []byte) ([]pullRequestFile, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	files := make([]pullRequestFile, 0)
	for {
		var payload json.RawMessage
		if err := decoder.Decode(&payload); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}

		var singlePage []pullRequestFile
		if err := json.Unmarshal(payload, &singlePage); err == nil {
			files = append(files, singlePage...)
			continue
		}

		var slurpedPages [][]pullRequestFile
		if err := json.Unmarshal(payload, &slurpedPages); err == nil {
			for _, page := range slurpedPages {
				files = append(files, page...)
			}
			continue
		}

		return nil, fmt.Errorf("unexpected pull request files payload")
	}

	return files, nil
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

func isInvalidAnchorCommandError(err error) bool {
	text := strings.ToLower(err.Error())
	if !strings.Contains(text, "422") {
		return false
	}
	return strings.Contains(text, "line must be part of the diff") ||
		strings.Contains(text, "start_line must be part of the diff") ||
		strings.Contains(text, "is outside the diff") ||
		strings.Contains(text, "is not part of the diff") ||
		strings.Contains(text, "pull_request_review_thread.path") ||
		strings.Contains(text, "path is missing")
}
