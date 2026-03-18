package gitlab

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"bentos-backend/adapter/outbound/commandrunner"
	"bentos-backend/domain"
	"bentos-backend/shared/toolinstall"
)

// CLIClient executes GitLab operations via the glab CLI.
type CLIClient struct {
	runner      commandrunner.Runner
	installer   *toolinstall.GlabInstaller
	authChecked bool
	host        string
}

// CLIClientConfig configures the GitLab CLI client.
type CLIClientConfig struct {
	Host string
}

// NewCLIClient creates a GitLab CLI API client.
func NewCLIClient() *CLIClient {
	return NewCLIClientWithConfig(CLIClientConfig{})
}

// NewCLIClientWithConfig creates a GitLab CLI API client with configuration.
func NewCLIClientWithConfig(cfg CLIClientConfig) *CLIClient {
	return &CLIClient{
		runner:    commandrunner.NewOSCommandRunner(),
		installer: toolinstall.NewGlabInstaller(nil),
		host:      strings.TrimSpace(cfg.Host),
	}
}

// ResolveRepository resolves the effective GitLab repository path.
func (c *CLIClient) ResolveRepository(ctx context.Context, repository string) (string, error) {
	repository = strings.TrimSpace(repository)
	if repository != "" {
		return repository, nil
	}
	if err := c.ensureAuth(ctx); err != nil {
		return "", err
	}

	result, err := c.runGlab(ctx, "repo", "view", "--json", "path_with_namespace")
	if err != nil {
		return "", fmt.Errorf("failed to resolve current GitLab repository: %w", formatCommandError(err, result))
	}
	var payload struct {
		PathWithNamespace string `json:"path_with_namespace"`
	}
	if err := json.Unmarshal(result.Stdout, &payload); err != nil {
		return "", err
	}
	payload.PathWithNamespace = strings.TrimSpace(payload.PathWithNamespace)
	if payload.PathWithNamespace == "" {
		return "", fmt.Errorf("failed to resolve current GitLab repository")
	}
	return payload.PathWithNamespace, nil
}

// GetPullRequestInfo loads title/body/base/head for a merge request.
func (c *CLIClient) GetPullRequestInfo(ctx context.Context, repository string, pullRequestNumber int) (domain.ChangeRequestInfo, error) {
	if pullRequestNumber <= 0 {
		return domain.ChangeRequestInfo{}, fmt.Errorf("merge request number must be positive")
	}
	if err := c.ensureAuth(ctx); err != nil {
		return domain.ChangeRequestInfo{}, err
	}
	resolvedRepo, err := c.ResolveRepository(ctx, repository)
	if err != nil {
		return domain.ChangeRequestInfo{}, err
	}

	endpoint := fmt.Sprintf("projects/%s/merge_requests/%d", url.PathEscape(resolvedRepo), pullRequestNumber)
	result, err := c.api(ctx, "GET", endpoint)
	if err != nil {
		return domain.ChangeRequestInfo{}, fmt.Errorf("failed to load merge request metadata: %w", err)
	}

	var payload struct {
		Title        string `json:"title"`
		Description  string `json:"description"`
		SourceBranch string `json:"source_branch"`
		DiffRefs     struct {
			BaseSHA  string `json:"base_sha"`
			StartSHA string `json:"start_sha"`
			HeadSHA  string `json:"head_sha"`
		} `json:"diff_refs"`
	}
	if err := json.Unmarshal(result, &payload); err != nil {
		return domain.ChangeRequestInfo{}, fmt.Errorf("failed to parse merge request metadata: %w", err)
	}

	base := strings.TrimSpace(payload.DiffRefs.BaseSHA)
	head := strings.TrimSpace(payload.DiffRefs.HeadSHA)
	if base == "" || head == "" {
		return domain.ChangeRequestInfo{}, fmt.Errorf("failed to resolve merge request base/head refs")
	}

	return domain.ChangeRequestInfo{
		Repository:  resolvedRepo,
		Number:      pullRequestNumber,
		Title:       strings.TrimSpace(payload.Title),
		Description: strings.TrimSpace(payload.Description),
		BaseRef:     base,
		HeadRef:     head,
		HeadRefName: strings.TrimSpace(payload.SourceBranch),
		StartRef:    strings.TrimSpace(payload.DiffRefs.StartSHA),
	}, nil
}

// GetIssue loads issue metadata for a repository issue.
func (c *CLIClient) GetIssue(ctx context.Context, repository string, issueNumber int) (domain.Issue, error) {
	if issueNumber <= 0 {
		return domain.Issue{}, fmt.Errorf("issue number must be positive")
	}
	if err := c.ensureAuth(ctx); err != nil {
		return domain.Issue{}, err
	}
	resolvedRepo, err := c.ResolveRepository(ctx, repository)
	if err != nil {
		return domain.Issue{}, err
	}

	endpoint := fmt.Sprintf("projects/%s/issues/%d", url.PathEscape(resolvedRepo), issueNumber)
	result, err := c.api(ctx, "GET", endpoint)
	if err != nil {
		return domain.Issue{}, fmt.Errorf("failed to load issue: %w", err)
	}
	var payload struct {
		Iid         int    `json:"iid"`
		Title       string `json:"title"`
		Description string `json:"description"`
		WebURL      string `json:"web_url"`
	}
	if err := json.Unmarshal(result, &payload); err != nil {
		return domain.Issue{}, fmt.Errorf("failed to parse issue metadata: %w", err)
	}
	return domain.Issue{
		Repository: resolvedRepo,
		Number:     payload.Iid,
		Title:      strings.TrimSpace(payload.Title),
		Body:       strings.TrimSpace(payload.Description),
		URL:        strings.TrimSpace(payload.WebURL),
	}, nil
}

// ListIssueComments loads issue comments for a repository issue.
func (c *CLIClient) ListIssueComments(ctx context.Context, repository string, issueNumber int) ([]domain.IssueComment, error) {
	if issueNumber <= 0 {
		return nil, fmt.Errorf("issue number must be positive")
	}
	if err := c.ensureAuth(ctx); err != nil {
		return nil, err
	}
	resolvedRepo, err := c.ResolveRepository(ctx, repository)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("projects/%s/issues/%d/notes", url.PathEscape(resolvedRepo), issueNumber)
	result, err := c.apiPaginated(ctx, endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to load issue comments: %w", err)
	}
	return parseNotes(result)
}

// ListChangeRequestComments loads issue-style comments for a merge request.
func (c *CLIClient) ListChangeRequestComments(ctx context.Context, repository string, pullRequestNumber int) ([]domain.IssueComment, error) {
	if pullRequestNumber <= 0 {
		return nil, fmt.Errorf("merge request number must be positive")
	}
	if err := c.ensureAuth(ctx); err != nil {
		return nil, err
	}
	resolvedRepo, err := c.ResolveRepository(ctx, repository)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("projects/%s/merge_requests/%d/notes", url.PathEscape(resolvedRepo), pullRequestNumber)
	result, err := c.apiPaginated(ctx, endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to load merge request comments: %w", err)
	}
	return parseNotes(result)
}

// GetIssueComment loads a single merge request note by ID.
func (c *CLIClient) GetIssueComment(ctx context.Context, repository string, pullRequestNumber int, commentID int64) (domain.IssueComment, error) {
	if commentID <= 0 {
		return domain.IssueComment{}, fmt.Errorf("comment id must be positive")
	}
	if pullRequestNumber <= 0 {
		return domain.IssueComment{}, fmt.Errorf("merge request number must be positive")
	}
	if err := c.ensureAuth(ctx); err != nil {
		return domain.IssueComment{}, err
	}
	resolvedRepo, err := c.ResolveRepository(ctx, repository)
	if err != nil {
		return domain.IssueComment{}, err
	}

	endpoint := fmt.Sprintf("projects/%s/merge_requests/%d/notes/%d", url.PathEscape(resolvedRepo), pullRequestNumber, commentID)
	result, err := c.api(ctx, "GET", endpoint)
	if err != nil {
		return domain.IssueComment{}, fmt.Errorf("failed to load merge request comment: %w", err)
	}
	return parseNote(result)
}

// GetReviewComment loads a single review comment by ID.
func (c *CLIClient) GetReviewComment(ctx context.Context, repository string, pullRequestNumber int, commentID int64) (domain.ReviewComment, error) {
	if commentID <= 0 {
		return domain.ReviewComment{}, fmt.Errorf("comment id must be positive")
	}
	if pullRequestNumber <= 0 {
		return domain.ReviewComment{}, fmt.Errorf("merge request number must be positive")
	}
	comments, err := c.ListReviewComments(ctx, repository, pullRequestNumber)
	if err != nil {
		return domain.ReviewComment{}, err
	}
	for _, comment := range comments {
		if comment.ID == commentID {
			return comment, nil
		}
	}
	return domain.ReviewComment{}, fmt.Errorf("failed to resolve review comment")
}

// ListReviewComments loads review comments for a merge request.
func (c *CLIClient) ListReviewComments(ctx context.Context, repository string, pullRequestNumber int) ([]domain.ReviewComment, error) {
	if pullRequestNumber <= 0 {
		return nil, fmt.Errorf("merge request number must be positive")
	}
	if err := c.ensureAuth(ctx); err != nil {
		return nil, err
	}
	resolvedRepo, err := c.ResolveRepository(ctx, repository)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("projects/%s/merge_requests/%d/discussions", url.PathEscape(resolvedRepo), pullRequestNumber)
	result, err := c.apiPaginated(ctx, endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to load merge request discussions: %w", err)
	}
	return parseReviewDiscussions(result)
}

// GetPullRequestReview loads a pull request review summary.
func (c *CLIClient) GetPullRequestReview(ctx context.Context, repository string, pullRequestNumber int, reviewID int64) (domain.ReviewSummary, error) {
	_ = ctx
	_ = repository
	_ = pullRequestNumber
	return domain.ReviewSummary{ID: reviewID}, nil
}

// CreateComment posts a comment to GitLab merge request.
func (c *CLIClient) CreateComment(ctx context.Context, repository string, pullRequestNumber int, body string) error {
	if pullRequestNumber <= 0 {
		return fmt.Errorf("merge request number must be positive")
	}
	if err := c.ensureAuth(ctx); err != nil {
		return err
	}
	resolvedRepo, err := c.ResolveRepository(ctx, repository)
	if err != nil {
		return err
	}

	endpoint := fmt.Sprintf("projects/%s/merge_requests/%d/notes", url.PathEscape(resolvedRepo), pullRequestNumber)
	_, err = c.apiWithFields(ctx, "POST", endpoint, []field{{name: "body", value: body, raw: true}})
	if err != nil {
		return fmt.Errorf("failed to create merge request comment: %w", err)
	}
	return nil
}

// CreateReviewComment posts a file-anchored review comment to GitLab.
func (c *CLIClient) CreateReviewComment(ctx context.Context, repository string, pullRequestNumber int, input domain.ReviewCommentInput) error {
	if pullRequestNumber <= 0 {
		return fmt.Errorf("merge request number must be positive")
	}
	if err := c.ensureAuth(ctx); err != nil {
		return err
	}
	resolvedRepo, err := c.ResolveRepository(ctx, repository)
	if err != nil {
		return err
	}

	mrInfo, err := c.GetPullRequestInfo(ctx, resolvedRepo, pullRequestNumber)
	if err != nil {
		return err
	}
	if strings.TrimSpace(mrInfo.StartRef) == "" {
		return fmt.Errorf("merge request diff refs are required")
	}

	fields := []field{
		{name: "body", value: input.Body, raw: true},
	}
	positionFields, _ := buildPositionFields(input, mrInfo)
	fields = append(fields, positionFields...)

	endpoint := fmt.Sprintf("projects/%s/merge_requests/%d/discussions", url.PathEscape(resolvedRepo), pullRequestNumber)
	if _, err := c.apiWithFields(ctx, "POST", endpoint, fields); err != nil {
		if isInvalidAnchorCommandError(err) {
			return &domain.InvalidAnchorError{Message: "invalid review comment anchor", Cause: err}
		}
		return fmt.Errorf("failed to create merge request review comment: %w", err)
	}
	return nil
}

// CreateReviewReply posts a reply to a review comment thread.
func (c *CLIClient) CreateReviewReply(ctx context.Context, repository string, pullRequestNumber int, commentID int64, body string) error {
	if pullRequestNumber <= 0 {
		return fmt.Errorf("merge request number must be positive")
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

	discussionID, err := c.findDiscussionID(ctx, resolvedRepo, pullRequestNumber, commentID)
	if err != nil {
		return err
	}
	endpoint := fmt.Sprintf("projects/%s/merge_requests/%d/discussions/%s/notes", url.PathEscape(resolvedRepo), pullRequestNumber, discussionID)
	_, err = c.apiWithFields(ctx, "POST", endpoint, []field{{name: "body", value: body, raw: true}})
	if err != nil {
		return fmt.Errorf("failed to create review reply: %w", err)
	}
	return nil
}

func (c *CLIClient) findDiscussionID(ctx context.Context, repository string, pullRequestNumber int, commentID int64) (string, error) {
	endpoint := fmt.Sprintf("projects/%s/merge_requests/%d/discussions", url.PathEscape(repository), pullRequestNumber)
	result, err := c.apiPaginated(ctx, endpoint)
	if err != nil {
		return "", fmt.Errorf("failed to load merge request discussions: %w", err)
	}

	discussions, err := parseDiscussions(result)
	if err != nil {
		return "", err
	}
	for _, discussion := range discussions {
		for _, note := range discussion.Notes {
			if note.ID == commentID {
				return discussion.ID, nil
			}
		}
	}
	return "", fmt.Errorf("failed to resolve review discussion")
}

func (c *CLIClient) ensureAuth(ctx context.Context) error {
	if c.authChecked {
		return nil
	}
	if err := c.ensureGlabInstalled(ctx); err != nil {
		return err
	}
	result, err := c.runGlab(ctx, "auth", "status")
	if err != nil {
		return fmt.Errorf("gitlab CLI is not authenticated; run `glab auth login` first: %w", formatCommandError(err, result))
	}
	c.authChecked = true
	return nil
}

func (c *CLIClient) ensureGlabInstalled(ctx context.Context) error {
	if c.installer == nil {
		c.installer = toolinstall.NewGlabInstaller(nil)
	}
	return c.installer.EnsureGlabInstalled(ctx)
}

type field struct {
	name  string
	value string
	raw   bool
}

func (c *CLIClient) api(ctx context.Context, method string, endpoint string) ([]byte, error) {
	args := []string{"api"}
	if strings.TrimSpace(method) != "" {
		args = append(args, "--method", method)
	}
	args = append(args, endpoint)
	result, err := c.runGlab(ctx, args...)
	if err != nil {
		return nil, formatCommandError(err, result)
	}
	return result.Stdout, nil
}

func (c *CLIClient) apiWithFields(ctx context.Context, method string, endpoint string, fields []field) ([]byte, error) {
	args := []string{"api"}
	if strings.TrimSpace(method) != "" {
		args = append(args, "--method", method)
	}
	for _, f := range fields {
		if f.raw {
			args = append(args, "--raw-field", f.name+"="+f.value)
		} else {
			args = append(args, "--field", f.name+"="+f.value)
		}
	}
	args = append(args, endpoint)
	result, err := c.runGlab(ctx, args...)
	if err != nil {
		return nil, formatCommandError(err, result)
	}
	return result.Stdout, nil
}

func (c *CLIClient) apiPaginated(ctx context.Context, endpoint string) ([]byte, error) {
	args := []string{"api", "--paginate", endpoint}
	result, err := c.runGlab(ctx, args...)
	if err != nil {
		return nil, formatCommandError(err, result)
	}
	return result.Stdout, nil
}

func (c *CLIClient) runGlab(ctx context.Context, args ...string) (commandrunner.Result, error) {
	fullArgs := make([]string, 0, len(args)+2)
	if strings.TrimSpace(c.host) != "" {
		fullArgs = append(fullArgs, "--hostname", c.host)
	}
	fullArgs = append(fullArgs, args...)
	return c.runner.Run(ctx, "glab", fullArgs...)
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

func parseNote(raw []byte) (domain.IssueComment, error) {
	var payload struct {
		ID        int64  `json:"id"`
		Body      string `json:"body"`
		CreatedAt string `json:"created_at"`
		System    bool   `json:"system"`
		Author    struct {
			Username string `json:"username"`
			Name     string `json:"name"`
		} `json:"author"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return domain.IssueComment{}, fmt.Errorf("failed to parse note: %w", err)
	}
	if payload.System {
		return domain.IssueComment{}, fmt.Errorf("system note")
	}
	createdAt, err := time.Parse(time.RFC3339, strings.TrimSpace(payload.CreatedAt))
	if err != nil {
		return domain.IssueComment{}, fmt.Errorf("failed to parse note timestamp: %w", err)
	}
	author := strings.TrimSpace(payload.Author.Username)
	if author == "" {
		author = strings.TrimSpace(payload.Author.Name)
	}
	return domain.IssueComment{
		ID:        payload.ID,
		Body:      payload.Body,
		Author:    domain.CommentAuthor{Login: author},
		CreatedAt: createdAt,
	}, nil
}

func parseNotes(raw []byte) ([]domain.IssueComment, error) {
	var payloads []struct {
		ID        int64  `json:"id"`
		Body      string `json:"body"`
		CreatedAt string `json:"created_at"`
		System    bool   `json:"system"`
		Author    struct {
			Username string `json:"username"`
			Name     string `json:"name"`
		} `json:"author"`
	}

	decoded, err := decodeJSONArrayDocuments[struct {
		ID        int64  `json:"id"`
		Body      string `json:"body"`
		CreatedAt string `json:"created_at"`
		System    bool   `json:"system"`
		Author    struct {
			Username string `json:"username"`
			Name     string `json:"name"`
		} `json:"author"`
	}](raw)
	if err != nil {
		return nil, fmt.Errorf("failed to parse notes: %w", err)
	}
	payloads = decoded

	comments := make([]domain.IssueComment, 0, len(payloads))
	for _, item := range payloads {
		if item.System {
			continue
		}
		createdAt, err := time.Parse(time.RFC3339, strings.TrimSpace(item.CreatedAt))
		if err != nil {
			return nil, fmt.Errorf("failed to parse note timestamp: %w", err)
		}
		author := strings.TrimSpace(item.Author.Username)
		if author == "" {
			author = strings.TrimSpace(item.Author.Name)
		}
		comments = append(comments, domain.IssueComment{
			ID:        item.ID,
			Body:      item.Body,
			Author:    domain.CommentAuthor{Login: author},
			CreatedAt: createdAt,
		})
	}
	return comments, nil
}

type discussion struct {
	ID    string           `json:"id"`
	Notes []discussionNote `json:"notes"`
}

type discussionNote struct {
	ID          int64  `json:"id"`
	Body        string `json:"body"`
	CreatedAt   string `json:"created_at"`
	InReplyToID int64  `json:"in_reply_to_id"`
	Author      struct {
		Username string `json:"username"`
		Name     string `json:"name"`
	} `json:"author"`
	Position *struct {
		BaseSHA  string `json:"base_sha"`
		StartSHA string `json:"start_sha"`
		HeadSHA  string `json:"head_sha"`
		NewPath  string `json:"new_path"`
		OldPath  string `json:"old_path"`
		NewLine  int    `json:"new_line"`
		OldLine  int    `json:"old_line"`
	} `json:"position"`
}

func parseDiscussions(raw []byte) ([]discussion, error) {
	decoded, err := decodeJSONArrayDocuments[discussion](raw)
	if err != nil {
		return nil, fmt.Errorf("failed to parse discussions: %w", err)
	}
	return decoded, nil
}

func parseReviewDiscussions(raw []byte) ([]domain.ReviewComment, error) {
	discussions, err := parseDiscussions(raw)
	if err != nil {
		return nil, err
	}

	comments := make([]domain.ReviewComment, 0)
	for _, discussion := range discussions {
		for _, note := range discussion.Notes {
			if note.Position == nil {
				continue
			}
			createdAt, err := time.Parse(time.RFC3339, strings.TrimSpace(note.CreatedAt))
			if err != nil {
				return nil, fmt.Errorf("failed to parse discussion note timestamp: %w", err)
			}
			author := strings.TrimSpace(note.Author.Username)
			if author == "" {
				author = strings.TrimSpace(note.Author.Name)
			}
			path := strings.TrimSpace(note.Position.NewPath)
			if path == "" {
				path = strings.TrimSpace(note.Position.OldPath)
			}
			line := note.Position.NewLine
			side := "RIGHT"
			originalLine := 0
			if line == 0 {
				line = note.Position.OldLine
				side = "LEFT"
				originalLine = line
			}
			comments = append(comments, domain.ReviewComment{
				ID:           note.ID,
				Body:         note.Body,
				Author:       domain.CommentAuthor{Login: author},
				CreatedAt:    createdAt,
				InReplyToID:  note.InReplyToID,
				Path:         path,
				Line:         line,
				OriginalLine: originalLine,
				Side:         side,
			})
		}
	}
	return comments, nil
}

func decodeJSONArrayDocuments[T any](raw []byte) ([]T, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var all []T
	for {
		var batch []T
		if err := decoder.Decode(&batch); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		all = append(all, batch...)
	}
	return all, nil
}

func isInvalidAnchorCommandError(err error) bool {
	message := strings.ToLower(err.Error())
	if !strings.Contains(message, "400") && !strings.Contains(message, "422") {
		return false
	}
	return strings.Contains(message, "line_code") ||
		strings.Contains(message, "position") ||
		strings.Contains(message, "diff") ||
		strings.Contains(message, "path")
}
