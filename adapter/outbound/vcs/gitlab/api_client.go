package gitlab

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/bentos-lab/peer/domain"
)

const defaultGitLabTimeout = 30 * time.Second

// APIClientConfig configures the GitLab API client.
type APIClientConfig struct {
	BaseURL string
	Token   string
}

// APIClient executes GitLab operations using a personal access token.
type APIClient struct {
	httpClient *http.Client
	baseURL    string
	token      string
	host       string
}

// NewAPIClient creates a GitLab API client using a personal access token.
func NewAPIClient(httpClient *http.Client, cfg APIClientConfig) (*APIClient, error) {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultGitLabTimeout}
	}
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("gitlab API base URL is required")
	}
	token := strings.TrimSpace(cfg.Token)
	if token == "" {
		return nil, fmt.Errorf("gitlab token is required")
	}
	host, err := extractHost(baseURL)
	if err != nil {
		return nil, err
	}
	return &APIClient{
		httpClient: httpClient,
		baseURL:    baseURL,
		token:      token,
		host:       host,
	}, nil
}

// GetPullRequestInfo loads title/body/base/head for a merge request.
func (c *APIClient) GetPullRequestInfo(ctx context.Context, repository string, pullRequestNumber int) (domain.ChangeRequestInfo, error) {
	if pullRequestNumber <= 0 {
		return domain.ChangeRequestInfo{}, fmt.Errorf("merge request number must be positive")
	}
	repository = strings.TrimSpace(repository)
	if repository == "" {
		return domain.ChangeRequestInfo{}, fmt.Errorf("repository is required")
	}
	endpoint := fmt.Sprintf("%s/projects/%s/merge_requests/%d", c.baseURL, url.PathEscape(repository), pullRequestNumber)
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
	if err := c.requestJSON(ctx, http.MethodGet, endpoint, nil, &payload); err != nil {
		return domain.ChangeRequestInfo{}, fmt.Errorf("failed to load merge request metadata: %w", err)
	}
	base := strings.TrimSpace(payload.DiffRefs.BaseSHA)
	head := strings.TrimSpace(payload.DiffRefs.HeadSHA)
	if base == "" || head == "" {
		return domain.ChangeRequestInfo{}, fmt.Errorf("failed to resolve merge request base/head refs")
	}
	return domain.ChangeRequestInfo{
		Repository:  repository,
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
func (c *APIClient) GetIssue(ctx context.Context, repository string, issueNumber int) (domain.Issue, error) {
	if issueNumber <= 0 {
		return domain.Issue{}, fmt.Errorf("issue number must be positive")
	}
	repository = strings.TrimSpace(repository)
	if repository == "" {
		return domain.Issue{}, fmt.Errorf("repository is required")
	}
	endpoint := fmt.Sprintf("%s/projects/%s/issues/%d", c.baseURL, url.PathEscape(repository), issueNumber)
	var payload struct {
		Iid         int    `json:"iid"`
		Title       string `json:"title"`
		Description string `json:"description"`
		WebURL      string `json:"web_url"`
	}
	if err := c.requestJSON(ctx, http.MethodGet, endpoint, nil, &payload); err != nil {
		return domain.Issue{}, fmt.Errorf("failed to load issue: %w", err)
	}
	return domain.Issue{
		Repository: repository,
		Number:     payload.Iid,
		Title:      strings.TrimSpace(payload.Title),
		Body:       strings.TrimSpace(payload.Description),
		URL:        strings.TrimSpace(payload.WebURL),
	}, nil
}

// ListIssueComments loads issue comments for a repository issue.
func (c *APIClient) ListIssueComments(ctx context.Context, repository string, issueNumber int) ([]domain.IssueComment, error) {
	if issueNumber <= 0 {
		return nil, fmt.Errorf("issue number must be positive")
	}
	repository = strings.TrimSpace(repository)
	if repository == "" {
		return nil, fmt.Errorf("repository is required")
	}
	endpoint := fmt.Sprintf("%s/projects/%s/issues/%d/notes", c.baseURL, url.PathEscape(repository), issueNumber)
	return c.loadNotes(ctx, endpoint)
}

// ListChangeRequestComments loads issue-style comments for a merge request.
func (c *APIClient) ListChangeRequestComments(ctx context.Context, repository string, pullRequestNumber int) ([]domain.IssueComment, error) {
	if pullRequestNumber <= 0 {
		return nil, fmt.Errorf("merge request number must be positive")
	}
	repository = strings.TrimSpace(repository)
	if repository == "" {
		return nil, fmt.Errorf("repository is required")
	}
	endpoint := fmt.Sprintf("%s/projects/%s/merge_requests/%d/notes", c.baseURL, url.PathEscape(repository), pullRequestNumber)
	return c.loadNotes(ctx, endpoint)
}

// ListReviewComments loads review comments for a merge request.
func (c *APIClient) ListReviewComments(ctx context.Context, repository string, pullRequestNumber int) ([]domain.ReviewComment, error) {
	if pullRequestNumber <= 0 {
		return nil, fmt.Errorf("merge request number must be positive")
	}
	repository = strings.TrimSpace(repository)
	if repository == "" {
		return nil, fmt.Errorf("repository is required")
	}
	endpoint := fmt.Sprintf("%s/projects/%s/merge_requests/%d/discussions", c.baseURL, url.PathEscape(repository), pullRequestNumber)
	return c.loadReviewDiscussions(ctx, endpoint)
}

// GetPullRequestReview loads a pull request review summary (not supported for GitLab).
func (c *APIClient) GetPullRequestReview(_ context.Context, _ string, _ int, reviewID int64) (domain.ReviewSummary, error) {
	return domain.ReviewSummary{ID: reviewID}, nil
}

// CreateComment posts a comment to a merge request.
func (c *APIClient) CreateComment(ctx context.Context, repository string, pullRequestNumber int, body string) error {
	if pullRequestNumber <= 0 {
		return fmt.Errorf("merge request number must be positive")
	}
	repository = strings.TrimSpace(repository)
	if repository == "" {
		return fmt.Errorf("repository is required")
	}
	endpoint := fmt.Sprintf("%s/projects/%s/merge_requests/%d/notes", c.baseURL, url.PathEscape(repository), pullRequestNumber)
	payload := map[string]string{"body": body}
	if err := c.requestJSON(ctx, http.MethodPost, endpoint, payload, nil); err != nil {
		return fmt.Errorf("failed to create merge request comment: %w", err)
	}
	return nil
}

// CreateReviewComment posts a file-anchored review comment to GitLab.
func (c *APIClient) CreateReviewComment(ctx context.Context, repository string, pullRequestNumber int, input domain.ReviewCommentInput) error {
	if pullRequestNumber <= 0 {
		return fmt.Errorf("merge request number must be positive")
	}
	repository = strings.TrimSpace(repository)
	if repository == "" {
		return fmt.Errorf("repository is required")
	}
	mrInfo, err := c.GetPullRequestInfo(ctx, repository, pullRequestNumber)
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
	endpoint := fmt.Sprintf("%s/projects/%s/merge_requests/%d/discussions", c.baseURL, url.PathEscape(repository), pullRequestNumber)
	if err := c.requestForm(ctx, http.MethodPost, endpoint, fields); err != nil {
		if isInvalidAnchorAPIError(err) {
			return &domain.InvalidAnchorError{Message: "invalid review comment anchor", Cause: err}
		}
		return fmt.Errorf("failed to create merge request review comment: %w", err)
	}
	return nil
}

// CreateReviewReply posts a reply to a review comment thread.
func (c *APIClient) CreateReviewReply(ctx context.Context, repository string, pullRequestNumber int, commentID int64, body string) error {
	if pullRequestNumber <= 0 {
		return fmt.Errorf("merge request number must be positive")
	}
	if commentID <= 0 {
		return fmt.Errorf("comment id must be positive")
	}
	repository = strings.TrimSpace(repository)
	if repository == "" {
		return fmt.Errorf("repository is required")
	}
	discussionID, err := c.findDiscussionID(ctx, repository, pullRequestNumber, commentID)
	if err != nil {
		return err
	}
	endpoint := fmt.Sprintf("%s/projects/%s/merge_requests/%d/discussions/%s/notes", c.baseURL, url.PathEscape(repository), pullRequestNumber, discussionID)
	payload := map[string]string{"body": body}
	if err := c.requestJSON(ctx, http.MethodPost, endpoint, payload, nil); err != nil {
		return fmt.Errorf("failed to create review reply: %w", err)
	}
	return nil
}

// HasMaintainerAccess returns true when the token user has maintainer+ access on project.
func (c *APIClient) HasMaintainerAccess(ctx context.Context, projectID int) (bool, string, error) {
	if projectID <= 0 {
		return false, "", fmt.Errorf("project id must be positive")
	}
	endpoint := fmt.Sprintf("%s/projects/%d", c.baseURL, projectID)
	var payload struct {
		PathWithNamespace string `json:"path_with_namespace"`
		Permissions       struct {
			ProjectAccess *struct {
				AccessLevel int `json:"access_level"`
			} `json:"project_access"`
			GroupAccess *struct {
				AccessLevel int `json:"access_level"`
			} `json:"group_access"`
		} `json:"permissions"`
	}
	if err := c.requestJSON(ctx, http.MethodGet, endpoint, nil, &payload); err != nil {
		return false, "", fmt.Errorf("failed to load project permissions: %w", err)
	}
	access := 0
	if payload.Permissions.ProjectAccess != nil {
		access = payload.Permissions.ProjectAccess.AccessLevel
	}
	if payload.Permissions.GroupAccess != nil && payload.Permissions.GroupAccess.AccessLevel > access {
		access = payload.Permissions.GroupAccess.AccessLevel
	}
	return access >= 40, strings.TrimSpace(payload.PathWithNamespace), nil
}

// ListProjectHooks loads hooks for a project.
func (c *APIClient) ListProjectHooks(ctx context.Context, projectID int) ([]ProjectHook, error) {
	if projectID <= 0 {
		return nil, fmt.Errorf("project id must be positive")
	}
	endpoint := fmt.Sprintf("%s/projects/%d/hooks", c.baseURL, projectID)
	var hooks []ProjectHook
	if err := c.requestJSON(ctx, http.MethodGet, endpoint, nil, &hooks); err != nil {
		return nil, fmt.Errorf("failed to load project hooks: %w", err)
	}
	return hooks, nil
}

// CreateProjectHook creates a webhook for a project.
func (c *APIClient) CreateProjectHook(ctx context.Context, projectID int, input HookInput) error {
	if projectID <= 0 {
		return fmt.Errorf("project id must be positive")
	}
	endpoint := fmt.Sprintf("%s/projects/%d/hooks", c.baseURL, projectID)
	return c.requestHook(ctx, http.MethodPost, endpoint, input)
}

// UpdateProjectHook updates a webhook for a project.
func (c *APIClient) UpdateProjectHook(ctx context.Context, projectID int, hookID int, input HookInput) error {
	if projectID <= 0 {
		return fmt.Errorf("project id must be positive")
	}
	if hookID <= 0 {
		return fmt.Errorf("hook id must be positive")
	}
	endpoint := fmt.Sprintf("%s/projects/%d/hooks/%d", c.baseURL, projectID, hookID)
	return c.requestHook(ctx, http.MethodPut, endpoint, input)
}

// ListUserEvents loads recent user events.
func (c *APIClient) ListUserEvents(ctx context.Context, after time.Time) ([]UserEvent, error) {
	endpoint := fmt.Sprintf("%s/events", c.baseURL)
	query := url.Values{}
	if !after.IsZero() {
		query.Set("after", after.UTC().Format("2006-01-02"))
	}
	query.Set("per_page", "100")
	endpoint = endpoint + "?" + query.Encode()
	raw, err := c.requestPaginated(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	events, err := decodeJSONArrayDocuments[UserEvent](raw)
	if err != nil {
		return nil, fmt.Errorf("failed to decode events: %w", err)
	}
	return events, nil
}

// BuildAuthenticatedCloneURL returns a GitLab HTTPS clone URL with PAT authentication.
func (c *APIClient) BuildAuthenticatedCloneURL(repository string) (string, error) {
	repository = strings.TrimSpace(repository)
	if repository == "" {
		return "", fmt.Errorf("repository is required")
	}
	token := strings.TrimSpace(c.token)
	if token == "" {
		return "", fmt.Errorf("token is required")
	}
	return fmt.Sprintf("https://oauth2:%s@%s/%s.git", url.PathEscape(token), c.host, repository), nil
}

// ProjectHook represents a GitLab project hook.
type ProjectHook struct {
	ID                  int    `json:"id"`
	URL                 string `json:"url"`
	Token               string `json:"token"`
	MergeRequestsEvents bool   `json:"merge_requests_events"`
	NoteEvents          bool   `json:"note_events"`
}

// HookInput represents hook settings.
type HookInput struct {
	URL                 string
	Token               string
	MergeRequestsEvents bool
	NoteEvents          bool
}

// UserEvent represents a GitLab user event.
type UserEvent struct {
	ActionName string    `json:"action_name"`
	CreatedAt  time.Time `json:"created_at"`
	ProjectID  int       `json:"project_id"`
	TargetType string    `json:"target_type"`
}

func (c *APIClient) requestHook(ctx context.Context, method string, endpoint string, input HookInput) error {
	fields := []field{
		{name: "url", value: input.URL, raw: true},
		{name: "token", value: input.Token, raw: true},
		{name: "merge_requests_events", value: strconv.FormatBool(input.MergeRequestsEvents), raw: true},
		{name: "note_events", value: strconv.FormatBool(input.NoteEvents), raw: true},
	}
	return c.requestForm(ctx, method, endpoint, fields)
}

func (c *APIClient) requestJSON(ctx context.Context, method string, endpoint string, payload any, out any) error {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return err
	}
	req.Header.Set("PRIVATE-TOKEN", c.token)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("gitlab api error: %s", strings.TrimSpace(string(raw)))
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(raw, out)
}

func (c *APIClient) requestForm(ctx context.Context, method string, endpoint string, fields []field) error {
	form := url.Values{}
	for _, f := range fields {
		if f.raw {
			form.Set(f.name, f.value)
		} else {
			form.Add(f.name, f.value)
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("PRIVATE-TOKEN", c.token)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("gitlab api error: %s", strings.TrimSpace(string(raw)))
	}
	return nil
}

func (c *APIClient) requestPaginated(ctx context.Context, endpoint string) ([]byte, error) {
	all := bytes.NewBuffer(nil)
	page := 1
	for {
		paged := endpoint
		separator := "?"
		if strings.Contains(endpoint, "?") {
			separator = "&"
		}
		paged = fmt.Sprintf("%s%spage=%d", endpoint, separator, page)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, paged, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("PRIVATE-TOKEN", c.token)
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		raw, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("gitlab api error: %s", strings.TrimSpace(string(raw)))
		}
		all.Write(raw)
		nextPage := strings.TrimSpace(resp.Header.Get("X-Next-Page"))
		if nextPage == "" {
			break
		}
		page++
	}
	return all.Bytes(), nil
}

func (c *APIClient) loadNotes(ctx context.Context, endpoint string) ([]domain.IssueComment, error) {
	raw, err := c.requestPaginated(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	payloads, err := decodeJSONArrayDocuments[struct {
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

func (c *APIClient) loadReviewDiscussions(ctx context.Context, endpoint string) ([]domain.ReviewComment, error) {
	raw, err := c.requestPaginated(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	discussions, err := decodeJSONArrayDocuments[discussion](raw)
	if err != nil {
		return nil, fmt.Errorf("failed to parse discussions: %w", err)
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

func (c *APIClient) findDiscussionID(ctx context.Context, repository string, pullRequestNumber int, commentID int64) (string, error) {
	endpoint := fmt.Sprintf("%s/projects/%s/merge_requests/%d/discussions", c.baseURL, url.PathEscape(repository), pullRequestNumber)
	raw, err := c.requestPaginated(ctx, endpoint)
	if err != nil {
		return "", fmt.Errorf("failed to load merge request discussions: %w", err)
	}
	discussions, err := decodeJSONArrayDocuments[discussion](raw)
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

func isInvalidAnchorAPIError(err error) bool {
	message := strings.ToLower(err.Error())
	if !strings.Contains(message, "400") && !strings.Contains(message, "422") {
		return false
	}
	return strings.Contains(message, "line_code") ||
		strings.Contains(message, "position") ||
		strings.Contains(message, "diff") ||
		strings.Contains(message, "path")
}

func extractHost(baseURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return "", err
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("gitlab API base URL must include host")
	}
	return parsed.Host, nil
}
