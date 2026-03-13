package github

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"bentos-backend/adapter/outbound/commandrunner"
	"bentos-backend/domain"
	"bentos-backend/shared/toolinstall"
)

// CLIClient executes GitHub operations via the gh CLI.
type CLIClient struct {
	runner      commandrunner.Runner
	installer   *toolinstall.Installer
	authChecked bool
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
	HeadRefName string
}

// PullRequestReviewSummary contains normalized pull request review metadata.
type PullRequestReviewSummary struct {
	ID    int64
	Body  string
	State string
	User  CommentAuthor
}

// NewCLIClient creates a GitHub CLI API client.
func NewCLIClient() *CLIClient {
	return &CLIClient{
		runner:    commandrunner.NewOSCommandRunner(),
		installer: toolinstall.NewInstaller(toolinstall.Config{}),
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

// CreateReviewReply posts a reply to a review comment thread.
func (c *CLIClient) CreateReviewReply(ctx context.Context, repository string, pullRequestNumber int, commentID int64, body string) error {
	if pullRequestNumber <= 0 {
		return fmt.Errorf("pull request number must be positive")
	}
	if commentID <= 0 {
		return fmt.Errorf("comment id must be positive")
	}
	if err := c.ensureAuth(ctx); err != nil {
		return err
	}

	resolvedRepo, err := c.ResolveRepository(ctx, repository)
	if err != nil {
		return err
	}

	args := []string{
		"api",
		"--method",
		"POST",
		fmt.Sprintf("repos/%s/pulls/%d/comments", resolvedRepo, pullRequestNumber),
		"--field",
		"body=" + body,
		"--field",
		"in_reply_to=" + strconv.FormatInt(commentID, 10),
	}
	result, err := c.runner.Run(ctx, "gh", args...)
	if err != nil {
		return fmt.Errorf("failed to create review reply: %w", formatCommandError(err, result))
	}
	return nil
}

// GetIssueComment loads a single issue comment by ID.
func (c *CLIClient) GetIssueComment(ctx context.Context, repository string, commentID int64) (IssueComment, error) {
	if commentID <= 0 {
		return IssueComment{}, fmt.Errorf("comment id must be positive")
	}
	if err := c.ensureAuth(ctx); err != nil {
		return IssueComment{}, err
	}
	resolvedRepo, err := c.ResolveRepository(ctx, repository)
	if err != nil {
		return IssueComment{}, err
	}

	result, err := c.runner.Run(
		ctx,
		"gh",
		"api",
		fmt.Sprintf("repos/%s/issues/comments/%d", resolvedRepo, commentID),
	)
	if err != nil {
		return IssueComment{}, fmt.Errorf("failed to load issue comment: %w", formatCommandError(err, result))
	}
	return parseIssueComment(result.Stdout)
}

// GetReviewComment loads a single review comment by ID.
func (c *CLIClient) GetReviewComment(ctx context.Context, repository string, commentID int64) (ReviewComment, error) {
	if commentID <= 0 {
		return ReviewComment{}, fmt.Errorf("comment id must be positive")
	}
	if err := c.ensureAuth(ctx); err != nil {
		return ReviewComment{}, err
	}
	resolvedRepo, err := c.ResolveRepository(ctx, repository)
	if err != nil {
		return ReviewComment{}, err
	}

	result, err := c.runner.Run(
		ctx,
		"gh",
		"api",
		fmt.Sprintf("repos/%s/pulls/comments/%d", resolvedRepo, commentID),
	)
	if err != nil {
		return ReviewComment{}, fmt.Errorf("failed to load review comment: %w", formatCommandError(err, result))
	}
	return parseReviewComment(result.Stdout)
}

// ListIssueComments loads issue comments for a pull request.
func (c *CLIClient) ListIssueComments(ctx context.Context, repository string, pullRequestNumber int) ([]IssueComment, error) {
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

	result, err := c.runner.Run(
		ctx,
		"gh",
		"api",
		fmt.Sprintf("repos/%s/issues/%d/comments", resolvedRepo, pullRequestNumber),
		"--paginate",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load issue comments: %w", formatCommandError(err, result))
	}
	return parseIssueComments(result.Stdout)
}

// ListReviewComments loads review comments for a pull request.
func (c *CLIClient) ListReviewComments(ctx context.Context, repository string, pullRequestNumber int) ([]ReviewComment, error) {
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

	result, err := c.runner.Run(
		ctx,
		"gh",
		"api",
		fmt.Sprintf("repos/%s/pulls/%d/comments", resolvedRepo, pullRequestNumber),
		"--paginate",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load review comments: %w", formatCommandError(err, result))
	}
	return parseReviewComments(result.Stdout)
}

// GetPullRequestReview loads a pull request review summary.
func (c *CLIClient) GetPullRequestReview(ctx context.Context, repository string, pullRequestNumber int, reviewID int64) (PullRequestReviewSummary, error) {
	if pullRequestNumber <= 0 {
		return PullRequestReviewSummary{}, fmt.Errorf("pull request number must be positive")
	}
	if reviewID <= 0 {
		return PullRequestReviewSummary{}, fmt.Errorf("review id must be positive")
	}
	if err := c.ensureAuth(ctx); err != nil {
		return PullRequestReviewSummary{}, err
	}
	resolvedRepo, err := c.ResolveRepository(ctx, repository)
	if err != nil {
		return PullRequestReviewSummary{}, err
	}

	result, err := c.runner.Run(
		ctx,
		"gh",
		"api",
		fmt.Sprintf("repos/%s/pulls/%d/reviews/%d", resolvedRepo, pullRequestNumber, reviewID),
	)
	if err != nil {
		return PullRequestReviewSummary{}, fmt.Errorf("failed to load pull request review: %w", formatCommandError(err, result))
	}
	return parsePullRequestReview(result.Stdout)
}

// ResolveRepository resolves the effective GitHub repository slug.
func (c *CLIClient) ResolveRepository(ctx context.Context, repository string) (string, error) {
	if strings.TrimSpace(repository) == "" {
		if err := c.ensureAuth(ctx); err != nil {
			return "", err
		}
	}
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
		HeadRefName: strings.TrimSpace(payload.Head.Ref),
	}, nil
}

// GetIssue loads issue metadata for a repository issue.
func (c *CLIClient) GetIssue(ctx context.Context, repository string, issueNumber int) (Issue, error) {
	if issueNumber <= 0 {
		return Issue{}, fmt.Errorf("issue number must be positive")
	}
	if err := c.ensureAuth(ctx); err != nil {
		return Issue{}, err
	}
	resolvedRepo, err := c.ResolveRepository(ctx, repository)
	if err != nil {
		return Issue{}, err
	}

	result, err := c.runner.Run(
		ctx,
		"gh",
		"api",
		fmt.Sprintf("repos/%s/issues/%d", resolvedRepo, issueNumber),
	)
	if err != nil {
		return Issue{}, fmt.Errorf("failed to load issue: %w", formatCommandError(err, result))
	}

	var payload struct {
		Number  int    `json:"number"`
		Title   string `json:"title"`
		Body    string `json:"body"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.Unmarshal(result.Stdout, &payload); err != nil {
		return Issue{}, fmt.Errorf("failed to parse issue metadata: %w", err)
	}

	return Issue{
		Repository: resolvedRepo,
		Number:     payload.Number,
		Title:      strings.TrimSpace(payload.Title),
		Body:       strings.TrimSpace(payload.Body),
		URL:        strings.TrimSpace(payload.HTMLURL),
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
	if c.authChecked {
		return nil
	}
	if err := c.ensureGhInstalled(ctx); err != nil {
		return err
	}
	result, err := c.runner.Run(ctx, "gh", "auth", "status")
	if err != nil {
		return fmt.Errorf("github CLI is not authenticated; run `gh auth login` first: %w", formatCommandError(err, result))
	}
	c.authChecked = true
	return nil
}

func (c *CLIClient) resolveRepository(ctx context.Context, repository string) (string, error) {
	if err := c.ensureGhInstalled(ctx); err != nil {
		return "", err
	}
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

func (c *CLIClient) ensureGhInstalled(ctx context.Context) error {
	if c.installer == nil {
		c.installer = toolinstall.NewInstaller(toolinstall.Config{})
	}
	return c.installer.EnsureGhInstalled(ctx)
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

func parseIssueComment(raw []byte) (IssueComment, error) {
	var payload struct {
		ID        int64  `json:"id"`
		Body      string `json:"body"`
		CreatedAt string `json:"created_at"`
		User      struct {
			Login string `json:"login"`
			Type  string `json:"type"`
		} `json:"user"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return IssueComment{}, fmt.Errorf("failed to parse issue comment: %w", err)
	}
	createdAt, err := time.Parse(time.RFC3339, strings.TrimSpace(payload.CreatedAt))
	if err != nil {
		return IssueComment{}, fmt.Errorf("failed to parse issue comment timestamp: %w", err)
	}
	return IssueComment{
		ID:        payload.ID,
		Body:      payload.Body,
		Author:    CommentAuthor{Login: payload.User.Login, Type: payload.User.Type},
		CreatedAt: createdAt,
	}, nil
}

func parseReviewComment(raw []byte) (ReviewComment, error) {
	var payload struct {
		ID                int64  `json:"id"`
		Body              string `json:"body"`
		CreatedAt         string `json:"created_at"`
		InReplyToID       int64  `json:"in_reply_to_id"`
		Path              string `json:"path"`
		DiffHunk          string `json:"diff_hunk"`
		Line              int    `json:"line"`
		OriginalLine      int    `json:"original_line"`
		StartLine         int    `json:"start_line"`
		OriginalStartLine int    `json:"original_start_line"`
		Side              string `json:"side"`
		StartSide         string `json:"start_side"`
		CommitID          string `json:"commit_id"`
		ReviewID          int64  `json:"pull_request_review_id"`
		User              struct {
			Login string `json:"login"`
			Type  string `json:"type"`
		} `json:"user"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ReviewComment{}, fmt.Errorf("failed to parse review comment: %w", err)
	}
	createdAt, err := time.Parse(time.RFC3339, strings.TrimSpace(payload.CreatedAt))
	if err != nil {
		return ReviewComment{}, fmt.Errorf("failed to parse review comment timestamp: %w", err)
	}
	return ReviewComment{
		ID:                payload.ID,
		Body:              payload.Body,
		Author:            CommentAuthor{Login: payload.User.Login, Type: payload.User.Type},
		CreatedAt:         createdAt,
		InReplyToID:       payload.InReplyToID,
		Path:              payload.Path,
		DiffHunk:          payload.DiffHunk,
		Line:              payload.Line,
		OriginalLine:      payload.OriginalLine,
		StartLine:         payload.StartLine,
		OriginalStartLine: payload.OriginalStartLine,
		Side:              payload.Side,
		StartSide:         payload.StartSide,
		CommitID:          payload.CommitID,
		ReviewID:          payload.ReviewID,
	}, nil
}

func parseIssueComments(raw []byte) ([]IssueComment, error) {
	var payload []struct {
		ID        int64  `json:"id"`
		Body      string `json:"body"`
		CreatedAt string `json:"created_at"`
		User      struct {
			Login string `json:"login"`
			Type  string `json:"type"`
		} `json:"user"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse issue comments: %w", err)
	}
	comments := make([]IssueComment, 0, len(payload))
	for _, item := range payload {
		createdAt, err := time.Parse(time.RFC3339, strings.TrimSpace(item.CreatedAt))
		if err != nil {
			return nil, fmt.Errorf("failed to parse issue comment timestamp: %w", err)
		}
		comments = append(comments, IssueComment{
			ID:        item.ID,
			Body:      item.Body,
			Author:    CommentAuthor{Login: item.User.Login, Type: item.User.Type},
			CreatedAt: createdAt,
		})
	}
	return comments, nil
}

func parseReviewComments(raw []byte) ([]ReviewComment, error) {
	var payload []struct {
		ID                int64  `json:"id"`
		Body              string `json:"body"`
		CreatedAt         string `json:"created_at"`
		InReplyToID       int64  `json:"in_reply_to_id"`
		Path              string `json:"path"`
		DiffHunk          string `json:"diff_hunk"`
		Line              int    `json:"line"`
		OriginalLine      int    `json:"original_line"`
		StartLine         int    `json:"start_line"`
		OriginalStartLine int    `json:"original_start_line"`
		Side              string `json:"side"`
		StartSide         string `json:"start_side"`
		CommitID          string `json:"commit_id"`
		ReviewID          int64  `json:"pull_request_review_id"`
		User              struct {
			Login string `json:"login"`
			Type  string `json:"type"`
		} `json:"user"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse review comments: %w", err)
	}
	comments := make([]ReviewComment, 0, len(payload))
	for _, item := range payload {
		createdAt, err := time.Parse(time.RFC3339, strings.TrimSpace(item.CreatedAt))
		if err != nil {
			return nil, fmt.Errorf("failed to parse review comment timestamp: %w", err)
		}
		comments = append(comments, ReviewComment{
			ID:                item.ID,
			Body:              item.Body,
			Author:            CommentAuthor{Login: item.User.Login, Type: item.User.Type},
			CreatedAt:         createdAt,
			InReplyToID:       item.InReplyToID,
			Path:              item.Path,
			DiffHunk:          item.DiffHunk,
			Line:              item.Line,
			OriginalLine:      item.OriginalLine,
			StartLine:         item.StartLine,
			OriginalStartLine: item.OriginalStartLine,
			Side:              item.Side,
			StartSide:         item.StartSide,
			CommitID:          item.CommitID,
			ReviewID:          item.ReviewID,
		})
	}
	return comments, nil
}

func parsePullRequestReview(raw []byte) (PullRequestReviewSummary, error) {
	var payload struct {
		ID    int64  `json:"id"`
		Body  string `json:"body"`
		State string `json:"state"`
		User  struct {
			Login string `json:"login"`
			Type  string `json:"type"`
		} `json:"user"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return PullRequestReviewSummary{}, fmt.Errorf("failed to parse pull request review: %w", err)
	}
	return PullRequestReviewSummary{
		ID:    payload.ID,
		Body:  strings.TrimSpace(payload.Body),
		State: strings.TrimSpace(payload.State),
		User:  CommentAuthor{Login: payload.User.Login, Type: payload.User.Type},
	}, nil
}
