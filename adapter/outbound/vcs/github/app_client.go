package github

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"bentos-backend/domain"
)

const githubAPIVersion = "2022-11-28"

// AppClientConfig contains GitHub App client settings.
type AppClientConfig struct {
	APIBaseURL string
	AppID      string
	PrivateKey string
}

type installationTokenCacheItem struct {
	Token     string
	ExpiresAt time.Time
}

// AppClient executes GitHub operations using GitHub App installation tokens.
type AppClient struct {
	httpClient    *http.Client
	apiBaseURL    string
	appID         string
	privateKey    *rsa.PrivateKey
	now           func() time.Time
	cacheMutex    sync.Mutex
	tokenByInstID map[string]installationTokenCacheItem
}

// NewAppClient creates a GitHub App API client.
func NewAppClient(httpClient *http.Client, cfg AppClientConfig) (*AppClient, error) {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	apiBaseURL := strings.TrimSpace(cfg.APIBaseURL)
	if apiBaseURL == "" {
		return nil, fmt.Errorf("github API base URL is required")
	}
	appID := strings.TrimSpace(cfg.AppID)
	if appID == "" {
		return nil, fmt.Errorf("github app ID is required")
	}
	privateKey, err := parseGitHubAppPrivateKey(cfg.PrivateKey)
	if err != nil {
		return nil, err
	}
	return &AppClient{
		httpClient:    httpClient,
		apiBaseURL:    strings.TrimRight(apiBaseURL, "/"),
		appID:         appID,
		privateKey:    privateKey,
		now:           time.Now,
		tokenByInstID: make(map[string]installationTokenCacheItem),
	}, nil
}

// GetPullRequestChangedFiles loads changed files for a pull request.
func (c *AppClient) GetPullRequestChangedFiles(ctx context.Context, repository string, pullRequestNumber int) ([]domain.ChangedFile, error) {
	if pullRequestNumber <= 0 {
		return nil, fmt.Errorf("pull request number must be positive")
	}
	repository = strings.TrimSpace(repository)
	if repository == "" {
		return nil, fmt.Errorf("repository is required")
	}
	token, err := c.installationAccessToken(ctx)
	if err != nil {
		return nil, err
	}

	changedFiles := make([]domain.ChangedFile, 0)
	page := 1
	for {
		endpoint := fmt.Sprintf("%s/repos/%s/pulls/%d/files?per_page=100&page=%d", c.apiBaseURL, repository, pullRequestNumber, page)
		var payload []pullRequestFile
		if err := c.requestJSON(ctx, token, http.MethodGet, endpoint, nil, &payload); err != nil {
			return nil, err
		}
		changedFiles = append(changedFiles, mapPullRequestFilesToChangedFiles(payload)...)

		if len(payload) < 100 {
			break
		}
		page++
	}

	return changedFiles, nil
}

// GetInstallationAccessToken resolves an installation token for the provided installation ID.
func (c *AppClient) GetInstallationAccessToken(ctx context.Context, installationID string) (string, error) {
	installationID = strings.TrimSpace(installationID)
	if installationID == "" {
		return "", fmt.Errorf("installation id is required")
	}
	return c.installationAccessToken(WithInstallationID(ctx, installationID))
}

// CreateComment posts a comment to GitHub.
func (c *AppClient) CreateComment(ctx context.Context, repository string, pullRequestNumber int, body string) error {
	if pullRequestNumber <= 0 {
		return fmt.Errorf("pull request number must be positive")
	}
	repository = strings.TrimSpace(repository)
	if repository == "" {
		return fmt.Errorf("repository is required")
	}
	token, err := c.installationAccessToken(ctx)
	if err != nil {
		return err
	}

	payload := map[string]string{"body": body}
	endpoint := fmt.Sprintf("%s/repos/%s/issues/%d/comments", c.apiBaseURL, repository, pullRequestNumber)
	if err := c.requestJSON(ctx, token, http.MethodPost, endpoint, payload, nil); err != nil {
		return fmt.Errorf("failed to create pull request comment: %w", err)
	}
	return nil
}

// CreateReviewComment posts a file-anchored review comment to GitHub.
func (c *AppClient) CreateReviewComment(ctx context.Context, repository string, pullRequestNumber int, input CreateReviewCommentInput) error {
	if pullRequestNumber <= 0 {
		return fmt.Errorf("pull request number must be positive")
	}
	repository = strings.TrimSpace(repository)
	if repository == "" {
		return fmt.Errorf("repository is required")
	}
	token, err := c.installationAccessToken(ctx)
	if err != nil {
		return err
	}

	headSHA, err := c.getPullRequestHeadSHA(ctx, token, repository, pullRequestNumber)
	if err != nil {
		return err
	}

	payload := map[string]any{
		"body":      input.Body,
		"path":      input.Path,
		"side":      "RIGHT",
		"commit_id": headSHA,
		"line":      input.EndLine,
	}
	if input.StartLine != input.EndLine {
		payload["start_line"] = input.StartLine
		payload["start_side"] = "RIGHT"
	}

	endpoint := fmt.Sprintf("%s/repos/%s/pulls/%d/comments", c.apiBaseURL, repository, pullRequestNumber)
	if err := c.requestJSON(ctx, token, http.MethodPost, endpoint, payload, nil); err != nil {
		if isInvalidAnchorAPIError(err) {
			return &InvalidAnchorError{
				Message: "invalid review comment anchor",
				Cause:   err,
			}
		}
		return fmt.Errorf("failed to create pull request review comment: %w", err)
	}

	return nil
}

// CreateReviewReply posts a reply to a review comment thread.
func (c *AppClient) CreateReviewReply(ctx context.Context, repository string, pullRequestNumber int, commentID int64, body string) error {
	if pullRequestNumber <= 0 {
		return fmt.Errorf("pull request number must be positive")
	}
	if commentID <= 0 {
		return fmt.Errorf("comment id must be positive")
	}
	repository = strings.TrimSpace(repository)
	if repository == "" {
		return fmt.Errorf("repository is required")
	}
	token, err := c.installationAccessToken(ctx)
	if err != nil {
		return err
	}

	payload := map[string]any{
		"body":        body,
		"in_reply_to": commentID,
	}
	endpoint := fmt.Sprintf("%s/repos/%s/pulls/%d/comments", c.apiBaseURL, repository, pullRequestNumber)
	if err := c.requestJSON(ctx, token, http.MethodPost, endpoint, payload, nil); err != nil {
		return fmt.Errorf("failed to create review reply: %w", err)
	}
	return nil
}

// GetIssueComment loads a single issue comment by ID.
func (c *AppClient) GetIssueComment(ctx context.Context, repository string, commentID int64) (IssueComment, error) {
	if commentID <= 0 {
		return IssueComment{}, fmt.Errorf("comment id must be positive")
	}
	repository = strings.TrimSpace(repository)
	if repository == "" {
		return IssueComment{}, fmt.Errorf("repository is required")
	}
	token, err := c.installationAccessToken(ctx)
	if err != nil {
		return IssueComment{}, err
	}

	var payload struct {
		ID        int64  `json:"id"`
		Body      string `json:"body"`
		CreatedAt string `json:"created_at"`
		User      struct {
			Login string `json:"login"`
			Type  string `json:"type"`
		} `json:"user"`
	}
	endpoint := fmt.Sprintf("%s/repos/%s/issues/comments/%d", c.apiBaseURL, repository, commentID)
	if err := c.requestJSON(ctx, token, http.MethodGet, endpoint, nil, &payload); err != nil {
		return IssueComment{}, fmt.Errorf("failed to load issue comment: %w", err)
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

// GetReviewComment loads a single review comment by ID.
func (c *AppClient) GetReviewComment(ctx context.Context, repository string, commentID int64) (ReviewComment, error) {
	if commentID <= 0 {
		return ReviewComment{}, fmt.Errorf("comment id must be positive")
	}
	repository = strings.TrimSpace(repository)
	if repository == "" {
		return ReviewComment{}, fmt.Errorf("repository is required")
	}
	token, err := c.installationAccessToken(ctx)
	if err != nil {
		return ReviewComment{}, err
	}

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
	endpoint := fmt.Sprintf("%s/repos/%s/pulls/comments/%d", c.apiBaseURL, repository, commentID)
	if err := c.requestJSON(ctx, token, http.MethodGet, endpoint, nil, &payload); err != nil {
		return ReviewComment{}, fmt.Errorf("failed to load review comment: %w", err)
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

// ListIssueComments loads issue comments for a pull request.
func (c *AppClient) ListIssueComments(ctx context.Context, repository string, pullRequestNumber int) ([]IssueComment, error) {
	if pullRequestNumber <= 0 {
		return nil, fmt.Errorf("pull request number must be positive")
	}
	repository = strings.TrimSpace(repository)
	if repository == "" {
		return nil, fmt.Errorf("repository is required")
	}
	token, err := c.installationAccessToken(ctx)
	if err != nil {
		return nil, err
	}

	comments := make([]IssueComment, 0)
	page := 1
	for {
		endpoint := fmt.Sprintf("%s/repos/%s/issues/%d/comments?per_page=100&page=%d", c.apiBaseURL, repository, pullRequestNumber, page)
		var payload []struct {
			ID        int64  `json:"id"`
			Body      string `json:"body"`
			CreatedAt string `json:"created_at"`
			User      struct {
				Login string `json:"login"`
				Type  string `json:"type"`
			} `json:"user"`
		}
		if err := c.requestJSON(ctx, token, http.MethodGet, endpoint, nil, &payload); err != nil {
			return nil, fmt.Errorf("failed to load issue comments: %w", err)
		}
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
		if len(payload) < 100 {
			break
		}
		page++
	}
	return comments, nil
}

// ListReviewComments loads review comments for a pull request.
func (c *AppClient) ListReviewComments(ctx context.Context, repository string, pullRequestNumber int) ([]ReviewComment, error) {
	if pullRequestNumber <= 0 {
		return nil, fmt.Errorf("pull request number must be positive")
	}
	repository = strings.TrimSpace(repository)
	if repository == "" {
		return nil, fmt.Errorf("repository is required")
	}
	token, err := c.installationAccessToken(ctx)
	if err != nil {
		return nil, err
	}

	comments := make([]ReviewComment, 0)
	page := 1
	for {
		endpoint := fmt.Sprintf("%s/repos/%s/pulls/%d/comments?per_page=100&page=%d", c.apiBaseURL, repository, pullRequestNumber, page)
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
		if err := c.requestJSON(ctx, token, http.MethodGet, endpoint, nil, &payload); err != nil {
			return nil, fmt.Errorf("failed to load review comments: %w", err)
		}
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
		if len(payload) < 100 {
			break
		}
		page++
	}
	return comments, nil
}

// GetPullRequestReview loads a pull request review summary.
func (c *AppClient) GetPullRequestReview(ctx context.Context, repository string, pullRequestNumber int, reviewID int64) (PullRequestReviewSummary, error) {
	if pullRequestNumber <= 0 {
		return PullRequestReviewSummary{}, fmt.Errorf("pull request number must be positive")
	}
	if reviewID <= 0 {
		return PullRequestReviewSummary{}, fmt.Errorf("review id must be positive")
	}
	repository = strings.TrimSpace(repository)
	if repository == "" {
		return PullRequestReviewSummary{}, fmt.Errorf("repository is required")
	}
	token, err := c.installationAccessToken(ctx)
	if err != nil {
		return PullRequestReviewSummary{}, err
	}

	var payload struct {
		ID    int64  `json:"id"`
		Body  string `json:"body"`
		State string `json:"state"`
		User  struct {
			Login string `json:"login"`
			Type  string `json:"type"`
		} `json:"user"`
	}
	endpoint := fmt.Sprintf("%s/repos/%s/pulls/%d/reviews/%d", c.apiBaseURL, repository, pullRequestNumber, reviewID)
	if err := c.requestJSON(ctx, token, http.MethodGet, endpoint, nil, &payload); err != nil {
		return PullRequestReviewSummary{}, fmt.Errorf("failed to load pull request review: %w", err)
	}
	return PullRequestReviewSummary{
		ID:    payload.ID,
		Body:  strings.TrimSpace(payload.Body),
		State: strings.TrimSpace(payload.State),
		User:  CommentAuthor{Login: payload.User.Login, Type: payload.User.Type},
	}, nil
}

// GetPullRequestInfo loads title/body/base/head for a pull request.
func (c *AppClient) GetPullRequestInfo(ctx context.Context, repository string, pullRequestNumber int) (PullRequestInfo, error) {
	if pullRequestNumber <= 0 {
		return PullRequestInfo{}, fmt.Errorf("pull request number must be positive")
	}
	repository = strings.TrimSpace(repository)
	if repository == "" {
		return PullRequestInfo{}, fmt.Errorf("repository is required")
	}
	token, err := c.installationAccessToken(ctx)
	if err != nil {
		return PullRequestInfo{}, err
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
	endpoint := fmt.Sprintf("%s/repos/%s/pulls/%d", c.apiBaseURL, repository, pullRequestNumber)
	if err := c.requestJSON(ctx, token, http.MethodGet, endpoint, nil, &payload); err != nil {
		return PullRequestInfo{}, fmt.Errorf("failed to load pull request metadata: %w", err)
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
		Repository:  repository,
		Number:      pullRequestNumber,
		Title:       strings.TrimSpace(payload.Title),
		Description: strings.TrimSpace(payload.Body),
		BaseRef:     base,
		HeadRef:     head,
		HeadRefName: strings.TrimSpace(payload.Head.Ref),
	}, nil
}

// GetIssue loads issue metadata for a repository issue.
func (c *AppClient) GetIssue(ctx context.Context, repository string, issueNumber int) (Issue, error) {
	if issueNumber <= 0 {
		return Issue{}, fmt.Errorf("issue number must be positive")
	}
	repository = strings.TrimSpace(repository)
	if repository == "" {
		return Issue{}, fmt.Errorf("repository is required")
	}
	token, err := c.installationAccessToken(ctx)
	if err != nil {
		return Issue{}, err
	}

	var payload struct {
		Number  int    `json:"number"`
		Title   string `json:"title"`
		Body    string `json:"body"`
		HTMLURL string `json:"html_url"`
	}
	endpoint := fmt.Sprintf("%s/repos/%s/issues/%d", c.apiBaseURL, repository, issueNumber)
	if err := c.requestJSON(ctx, token, http.MethodGet, endpoint, nil, &payload); err != nil {
		return Issue{}, fmt.Errorf("failed to load issue: %w", err)
	}
	return Issue{
		Repository: repository,
		Number:     payload.Number,
		Title:      strings.TrimSpace(payload.Title),
		Body:       strings.TrimSpace(payload.Body),
		URL:        strings.TrimSpace(payload.HTMLURL),
	}, nil
}

func (c *AppClient) getPullRequestHeadSHA(ctx context.Context, token string, repository string, pullRequestNumber int) (string, error) {
	var payload struct {
		Head struct {
			SHA string `json:"sha"`
		} `json:"head"`
	}
	endpoint := fmt.Sprintf("%s/repos/%s/pulls/%d", c.apiBaseURL, repository, pullRequestNumber)
	if err := c.requestJSON(ctx, token, http.MethodGet, endpoint, nil, &payload); err != nil {
		return "", fmt.Errorf("failed to load pull request metadata: %w", err)
	}
	payload.Head.SHA = strings.TrimSpace(payload.Head.SHA)
	if payload.Head.SHA == "" {
		return "", fmt.Errorf("failed to resolve pull request head commit")
	}
	return payload.Head.SHA, nil
}

func (c *AppClient) installationAccessToken(ctx context.Context) (string, error) {
	installationID := installationIDFromContext(ctx)
	if installationID == "" {
		return "", fmt.Errorf("missing github app installation id in context")
	}

	c.cacheMutex.Lock()
	if item, ok := c.tokenByInstID[installationID]; ok && c.now().Before(item.ExpiresAt.Add(-time.Minute)) {
		c.cacheMutex.Unlock()
		return item.Token, nil
	}
	c.cacheMutex.Unlock()

	appJWT, err := c.createAppJWT()
	if err != nil {
		return "", err
	}

	token, expiresAt, err := c.createInstallationToken(ctx, installationID, appJWT)
	if err != nil {
		return "", err
	}

	c.cacheMutex.Lock()
	c.tokenByInstID[installationID] = installationTokenCacheItem{
		Token:     token,
		ExpiresAt: expiresAt,
	}
	c.cacheMutex.Unlock()

	return token, nil
}

func (c *AppClient) createInstallationToken(ctx context.Context, installationID string, appJWT string) (string, time.Time, error) {
	endpoint := fmt.Sprintf("%s/app/installations/%s/access_tokens", c.apiBaseURL, installationID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, http.NoBody)
	if err != nil {
		return "", time.Time{}, err
	}
	req.Header.Set("Authorization", "Bearer "+appJWT)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", githubAPIVersion)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", time.Time{}, err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", time.Time{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", time.Time{}, fmt.Errorf("github API request failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}

	var payload struct {
		Token     string `json:"token"`
		ExpiresAt string `json:"expires_at"`
	}
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return "", time.Time{}, err
	}
	payload.Token = strings.TrimSpace(payload.Token)
	if payload.Token == "" {
		return "", time.Time{}, fmt.Errorf("github installation token is empty")
	}
	expiresAt, err := time.Parse(time.RFC3339, payload.ExpiresAt)
	if err != nil {
		return "", time.Time{}, err
	}
	return payload.Token, expiresAt, nil
}

func (c *AppClient) createAppJWT() (string, error) {
	now := c.now().UTC()
	claims := map[string]any{
		"iat": now.Add(-30 * time.Second).Unix(),
		"exp": now.Add(9 * time.Minute).Unix(),
		"iss": c.appID,
	}
	headerJSON, err := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT"})
	if err != nil {
		return "", err
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	encodedHeader := base64.RawURLEncoding.EncodeToString(headerJSON)
	encodedClaims := base64.RawURLEncoding.EncodeToString(claimsJSON)
	signingInput := encodedHeader + "." + encodedClaims

	hashed := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, c.privateKey, crypto.SHA256, hashed[:])
	if err != nil {
		return "", err
	}

	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func (c *AppClient) requestJSON(ctx context.Context, token string, method string, endpoint string, requestBody any, out any) error {
	var body io.Reader
	if requestBody != nil {
		encodedBody, err := json.Marshal(requestBody)
		if err != nil {
			return err
		}
		body = bytes.NewReader(encodedBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", githubAPIVersion)
	if requestBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("github API request failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}
	if out == nil || len(responseBody) == 0 {
		return nil
	}
	return json.Unmarshal(responseBody, out)
}

func parseGitHubAppPrivateKey(raw string) (*rsa.PrivateKey, error) {
	resolvedRaw, err := resolveGitHubAppPrivateKeyRaw(raw)
	if err != nil {
		return nil, err
	}
	resolvedRaw = strings.ReplaceAll(resolvedRaw, `\n`, "\n")
	if resolvedRaw == "" {
		return nil, fmt.Errorf("github app private key is required")
	}

	block, _ := pem.Decode([]byte(resolvedRaw))
	if block == nil {
		return nil, fmt.Errorf("failed to decode github app private key PEM")
	}

	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse github app private key: %w", err)
	}
	privateKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("github app private key must be RSA")
	}
	return privateKey, nil
}

func resolveGitHubAppPrivateKeyRaw(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("github app private key is required")
	}

	if info, err := os.Stat(raw); err == nil {
		if info != nil {
			content, readErr := os.ReadFile(raw)
			if readErr != nil {
				return "", fmt.Errorf("failed to read github app private key file %q: %w", raw, readErr)
			}
			return strings.TrimSpace(string(content)), nil
		}
	}

	return raw, nil
}
