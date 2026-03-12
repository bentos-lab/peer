package github

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"bentos-backend/adapter/inbound/http/background"
	inboundlogging "bentos-backend/adapter/inbound/logging"
	githubvcs "bentos-backend/adapter/outbound/vcs/github"
	"bentos-backend/domain"
	"bentos-backend/shared/logger/stdlogger"
	"bentos-backend/shared/text"
	"bentos-backend/usecase"
)

const backgroundReviewTimeout = 10 * time.Minute
const backgroundReplyCommentTimeout = 10 * time.Minute

type pullRequestEvent struct {
	Action       string `json:"action"`
	Installation struct {
		ID int64 `json:"id"`
	} `json:"installation"`
	Repository struct {
		FullName string `json:"full_name"`
		CloneURL string `json:"clone_url"`
	} `json:"repository"`
	PullRequest struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		Body   string `json:"body"`
		Base   struct {
			Ref string `json:"ref"`
			SHA string `json:"sha"`
		} `json:"base"`
		Head struct {
			Ref string `json:"ref"`
			SHA string `json:"sha"`
		} `json:"head"`
	} `json:"pull_request"`
}

type issueCommentEvent struct {
	Action       string `json:"action"`
	Installation struct {
		ID int64 `json:"id"`
	} `json:"installation"`
	Repository struct {
		FullName string `json:"full_name"`
		CloneURL string `json:"clone_url"`
	} `json:"repository"`
	Issue struct {
		Number      int `json:"number"`
		PullRequest any `json:"pull_request"`
	} `json:"issue"`
	Comment struct {
		ID   int64  `json:"id"`
		Body string `json:"body"`
		User struct {
			Login string `json:"login"`
			Type  string `json:"type"`
		} `json:"user"`
	} `json:"comment"`
}

type reviewCommentEvent struct {
	Action       string `json:"action"`
	Installation struct {
		ID int64 `json:"id"`
	} `json:"installation"`
	Repository struct {
		FullName string `json:"full_name"`
		CloneURL string `json:"clone_url"`
	} `json:"repository"`
	PullRequest struct {
		Number int `json:"number"`
	} `json:"pull_request"`
	Comment struct {
		ID          int64  `json:"id"`
		Body        string `json:"body"`
		InReplyToID int64  `json:"in_reply_to_id"`
		User        struct {
			Login string `json:"login"`
			Type  string `json:"type"`
		} `json:"user"`
	} `json:"comment"`
}

// InstallationTokenProvider resolves installation access tokens.
type InstallationTokenProvider interface {
	GetInstallationAccessToken(ctx context.Context, installationID string) (string, error)
}

// CommentClient provides PR/comment metadata for replycomment handling.
type CommentClient interface {
	InstallationTokenProvider
	GetPullRequestInfo(ctx context.Context, repository string, pullRequestNumber int) (githubvcs.PullRequestInfo, error)
	GetPullRequestReview(ctx context.Context, repository string, pullRequestNumber int, reviewID int64) (githubvcs.PullRequestReviewSummary, error)
	ListIssueComments(ctx context.Context, repository string, pullRequestNumber int) ([]githubvcs.IssueComment, error)
	ListReviewComments(ctx context.Context, repository string, pullRequestNumber int) ([]githubvcs.ReviewComment, error)
}

// Handler receives GitHub webhook events and triggers review.
type Handler struct {
	reviewer          usecase.ChangeRequestUseCase
	replyer           usecase.ReplyCommentUseCase
	tokenProvider     CommentClient
	logger            usecase.Logger
	webhookSecret     string
	replyTriggerName  string
	enableOverview    bool
	enableSuggestions bool
}

// NewHandler creates a GitHub webhook handler.
func NewHandler(
	changeRequestUseCase usecase.ChangeRequestUseCase,
	replyCommentUseCase usecase.ReplyCommentUseCase,
	tokenProvider CommentClient,
	logger usecase.Logger,
	webhookSecret string,
	replyTriggerName string,
	enableOverview bool,
	enableSuggestions bool,
) *Handler {
	if logger == nil {
		logger = stdlogger.Nop()
	}
	return &Handler{
		reviewer:          changeRequestUseCase,
		replyer:           replyCommentUseCase,
		tokenProvider:     tokenProvider,
		logger:            logger,
		webhookSecret:     strings.TrimSpace(webhookSecret),
		replyTriggerName:  strings.TrimSpace(replyTriggerName),
		enableOverview:    enableOverview,
		enableSuggestions: enableSuggestions,
	}
}

// ServeHTTP handles pull_request events and starts review usecase.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	eventType := strings.TrimSpace(r.Header.Get("X-GitHub-Event"))
	if eventType == "" {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	if !h.verifySignature(strings.TrimSpace(r.Header.Get("X-Hub-Signature-256")), body) {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	switch strings.ToLower(eventType) {
	case "pull_request":
		h.handlePullRequestEvent(w, r, body)
	case "issue_comment":
		h.handleIssueCommentEvent(w, r, body)
	case "pull_request_review_comment":
		h.handleReviewCommentEvent(w, r, body)
	default:
		w.WriteHeader(http.StatusAccepted)
	}
}

func (h *Handler) handlePullRequestEvent(w http.ResponseWriter, r *http.Request, body []byte) {
	var event pullRequestEvent
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&event); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	if !isValidPullRequestEvent(event) {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	if !isReviewTriggerAction(event.Action) {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	if err := h.ensureInstallation(event.Installation.ID, event.Repository.FullName, event.PullRequest.Number); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	installationID := strconv.FormatInt(event.Installation.ID, 10)
	installationToken, err := h.tokenProvider.GetInstallationAccessToken(r.Context(), installationID)
	if err != nil {
		h.logger.Errorf("GitHub webhook failed to resolve installation token.")
		h.logger.Debugf("Repository is %q and change request number is %d.", event.Repository.FullName, event.PullRequest.Number)
		h.logger.Debugf("Failure details: %v.", err)
		http.Error(w, "failed to resolve installation token", http.StatusBadGateway)
		return
	}
	repoURL, err := buildAuthenticatedCloneURL(event.Repository.CloneURL, installationToken)
	if err != nil {
		h.logger.Errorf("GitHub webhook failed to build repository clone URL.")
		h.logger.Debugf("Repository is %q and change request number is %d.", event.Repository.FullName, event.PullRequest.Number)
		h.logger.Debugf("Failure details: %v.", err)
		http.Error(w, "invalid repository clone URL", http.StatusBadRequest)
		return
	}

	base := strings.TrimSpace(event.PullRequest.Base.SHA)
	if base == "" {
		base = strings.TrimSpace(event.PullRequest.Base.Ref)
	}
	head := strings.TrimSpace(event.PullRequest.Head.SHA)
	if head == "" {
		head = strings.TrimSpace(event.PullRequest.Head.Ref)
	}

	request := usecase.ChangeRequestRequest{
		Repository:          event.Repository.FullName,
		RepoURL:             repoURL,
		ChangeRequestNumber: event.PullRequest.Number,
		Title:               event.PullRequest.Title,
		Description:         event.PullRequest.Body,
		Base:                base,
		Head:                head,
		EnableOverview:      h.enableOverview && isInitialPROpenedAction(event.Action),
		EnableSuggestions:   h.enableSuggestions,
		OverviewExplicit:    false,
		SuggestionsExplicit: false,
		Metadata: map[string]string{
			"action": event.Action,
		},
	}

	background.RunReviewAsync(
		h.logger,
		"GitHub",
		event.Action,
		request,
		backgroundReviewTimeout,
		func(ctx context.Context) context.Context {
			return githubvcs.WithInstallationID(ctx, installationID)
		},
		func(ctx context.Context, req usecase.ChangeRequestRequest) error {
			_, err := h.reviewer.Execute(ctx, req)
			return err
		},
	)

	h.logger.Infof("GitHub webhook review request was accepted.")
	h.logger.Debugf("Repository is %q and change request number is %d.", request.Repository, request.ChangeRequestNumber)
	h.logger.Debugf("Webhook action is %q.", event.Action)
	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) handleIssueCommentEvent(w http.ResponseWriter, r *http.Request, body []byte) {
	if h.replyer == nil {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	var event issueCommentEvent
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&event); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	if !isValidIssueCommentEvent(event) {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	if !strings.EqualFold(strings.TrimSpace(event.Action), "created") {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	if !text.ContainsTrigger(event.Comment.Body, h.replyTriggerName) {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	if isBotAuthor(event.Comment.User.Type) {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	if err := h.ensureInstallation(event.Installation.ID, event.Repository.FullName, event.Issue.Number); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	installationID := strconv.FormatInt(event.Installation.ID, 10)
	installationToken, err := h.tokenProvider.GetInstallationAccessToken(r.Context(), installationID)
	if err != nil {
		h.logger.Errorf("GitHub webhook failed to resolve installation token.")
		h.logger.Debugf("Repository is %q and change request number is %d.", event.Repository.FullName, event.Issue.Number)
		h.logger.Debugf("Failure details: %v.", err)
		http.Error(w, "failed to resolve installation token", http.StatusBadGateway)
		return
	}
	repoURL, err := buildAuthenticatedCloneURL(event.Repository.CloneURL, installationToken)
	if err != nil {
		h.logger.Errorf("GitHub webhook failed to build repository clone URL.")
		h.logger.Debugf("Repository is %q and change request number is %d.", event.Repository.FullName, event.Issue.Number)
		h.logger.Debugf("Failure details: %v.", err)
		http.Error(w, "invalid repository clone URL", http.StatusBadRequest)
		return
	}

	ctx := githubvcs.WithInstallationID(r.Context(), installationID)
	prInfo, err := h.tokenProvider.GetPullRequestInfo(ctx, event.Repository.FullName, event.Issue.Number)
	if err != nil {
		http.Error(w, "failed to resolve pull request info", http.StatusBadGateway)
		return
	}
	thread, err := buildIssueThreadForWebhook(ctx, h.tokenProvider, event.Repository.FullName, event.Issue.Number, event.Comment.ID, prInfo)
	if err != nil {
		http.Error(w, "failed to load comment thread", http.StatusBadGateway)
		return
	}

	request := usecase.ReplyCommentRequest{
		Repository:          event.Repository.FullName,
		RepoURL:             repoURL,
		ChangeRequestNumber: event.Issue.Number,
		Title:               prInfo.Title,
		Description:         prInfo.Description,
		Base:                prInfo.BaseRef,
		Head:                prInfo.HeadRef,
		CommentID:           event.Comment.ID,
		CommentKind:         domain.CommentKindIssue,
		Question:            text.StripTrigger(event.Comment.Body, h.replyTriggerName),
		Thread:              thread,
		Publish:             true,
		Metadata: map[string]string{
			"action": event.Action,
		},
	}

	background.RunReplyCommentAsync(
		h.logger,
		"GitHub",
		event.Action,
		request,
		backgroundReplyCommentTimeout,
		func(ctx context.Context) context.Context {
			return githubvcs.WithInstallationID(ctx, installationID)
		},
		func(ctx context.Context, req usecase.ReplyCommentRequest) error {
			inboundlogging.LogReplyCommentInputSnapshot(h.logger, "webhook", event.Action, req)
			_, err := h.replyer.Execute(ctx, req)
			return err
		},
	)

	h.logger.Infof("GitHub webhook replycomment request was accepted.")
	h.logger.Debugf("Repository is %q and change request number is %d.", request.Repository, request.ChangeRequestNumber)
	h.logger.Debugf("Webhook action is %q.", event.Action)
	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) handleReviewCommentEvent(w http.ResponseWriter, r *http.Request, body []byte) {
	if h.replyer == nil {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	var event reviewCommentEvent
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&event); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	if !isValidReviewCommentEvent(event) {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	if !strings.EqualFold(strings.TrimSpace(event.Action), "created") {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	if !text.ContainsTrigger(event.Comment.Body, h.replyTriggerName) {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	if isBotAuthor(event.Comment.User.Type) {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	if err := h.ensureInstallation(event.Installation.ID, event.Repository.FullName, event.PullRequest.Number); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	installationID := strconv.FormatInt(event.Installation.ID, 10)
	installationToken, err := h.tokenProvider.GetInstallationAccessToken(r.Context(), installationID)
	if err != nil {
		h.logger.Errorf("GitHub webhook failed to resolve installation token.")
		h.logger.Debugf("Repository is %q and change request number is %d.", event.Repository.FullName, event.PullRequest.Number)
		h.logger.Debugf("Failure details: %v.", err)
		http.Error(w, "failed to resolve installation token", http.StatusBadGateway)
		return
	}
	repoURL, err := buildAuthenticatedCloneURL(event.Repository.CloneURL, installationToken)
	if err != nil {
		h.logger.Errorf("GitHub webhook failed to build repository clone URL.")
		h.logger.Debugf("Repository is %q and change request number is %d.", event.Repository.FullName, event.PullRequest.Number)
		h.logger.Debugf("Failure details: %v.", err)
		http.Error(w, "invalid repository clone URL", http.StatusBadRequest)
		return
	}

	ctx := githubvcs.WithInstallationID(r.Context(), installationID)
	prInfo, err := h.tokenProvider.GetPullRequestInfo(ctx, event.Repository.FullName, event.PullRequest.Number)
	if err != nil {
		http.Error(w, "failed to resolve pull request info", http.StatusBadGateway)
		return
	}
	thread, err := buildReviewThreadForWebhook(ctx, h.tokenProvider, event.Repository.FullName, event.PullRequest.Number, event.Comment.ID)
	if err != nil {
		http.Error(w, "failed to load comment thread", http.StatusBadGateway)
		return
	}

	request := usecase.ReplyCommentRequest{
		Repository:          event.Repository.FullName,
		RepoURL:             repoURL,
		ChangeRequestNumber: event.PullRequest.Number,
		Title:               prInfo.Title,
		Description:         prInfo.Description,
		Base:                prInfo.BaseRef,
		Head:                prInfo.HeadRef,
		CommentID:           event.Comment.ID,
		CommentKind:         domain.CommentKindReview,
		Question:            text.StripTrigger(event.Comment.Body, h.replyTriggerName),
		Thread:              thread,
		Publish:             true,
		Metadata: map[string]string{
			"action": event.Action,
		},
	}

	background.RunReplyCommentAsync(
		h.logger,
		"GitHub",
		event.Action,
		request,
		backgroundReplyCommentTimeout,
		func(ctx context.Context) context.Context {
			return githubvcs.WithInstallationID(ctx, installationID)
		},
		func(ctx context.Context, req usecase.ReplyCommentRequest) error {
			inboundlogging.LogReplyCommentInputSnapshot(h.logger, "webhook", event.Action, req)
			_, err := h.replyer.Execute(ctx, req)
			return err
		},
	)

	h.logger.Infof("GitHub webhook replycomment request was accepted.")
	h.logger.Debugf("Repository is %q and change request number is %d.", request.Repository, request.ChangeRequestNumber)
	h.logger.Debugf("Webhook action is %q.", event.Action)
	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) ensureInstallation(installationID int64, repository string, prNumber int) error {
	if installationID <= 0 {
		h.logger.Errorf("GitHub webhook payload is missing installation id.")
		h.logger.Debugf("Repository is %q and change request number is %d.", repository, prNumber)
		return errors.New("missing installation id")
	}
	if h.tokenProvider == nil {
		h.logger.Errorf("GitHub webhook token provider is not configured.")
		return errors.New("token provider is not configured")
	}
	return nil
}

func isValidPullRequestEvent(event pullRequestEvent) bool {
	return strings.TrimSpace(event.Repository.FullName) != "" &&
		strings.TrimSpace(event.Repository.CloneURL) != "" &&
		event.PullRequest.Number > 0 &&
		strings.TrimSpace(event.PullRequest.Base.Ref) != "" &&
		strings.TrimSpace(event.PullRequest.Head.Ref) != ""
}

func isReviewTriggerAction(action string) bool {
	switch strings.TrimSpace(strings.ToLower(action)) {
	case "opened", "synchronize", "reopened":
		return true
	default:
		return false
	}
}

func isValidIssueCommentEvent(event issueCommentEvent) bool {
	return strings.TrimSpace(event.Repository.FullName) != "" &&
		strings.TrimSpace(event.Repository.CloneURL) != "" &&
		event.Issue.Number > 0 &&
		event.Issue.PullRequest != nil &&
		event.Comment.ID > 0
}

func isValidReviewCommentEvent(event reviewCommentEvent) bool {
	return strings.TrimSpace(event.Repository.FullName) != "" &&
		strings.TrimSpace(event.Repository.CloneURL) != "" &&
		event.PullRequest.Number > 0 &&
		event.Comment.ID > 0
}

func isBotAuthor(authorType string) bool {
	return strings.EqualFold(strings.TrimSpace(authorType), "bot")
}

func buildIssueThreadForWebhook(ctx context.Context, client CommentClient, repository string, prNumber int, commentID int64, prInfo githubvcs.PullRequestInfo) (domain.CommentThread, error) {
	comments, err := client.ListIssueComments(ctx, repository, prNumber)
	if err != nil {
		return domain.CommentThread{}, err
	}
	threadComments := make([]domain.Comment, 0, len(comments))
	for _, comment := range comments {
		threadComments = append(threadComments, comment.ToDomain())
	}
	sort.Slice(threadComments, func(i, j int) bool {
		return threadComments[i].CreatedAt.Before(threadComments[j].CreatedAt)
	})
	return domain.CommentThread{
		Kind:     domain.CommentKindIssue,
		RootID:   commentID,
		Context:  buildIssueThreadContext(prInfo),
		Comments: threadComments,
	}, nil
}

func buildReviewThreadForWebhook(ctx context.Context, client CommentClient, repository string, prNumber int, commentID int64) (domain.CommentThread, error) {
	comments, err := client.ListReviewComments(ctx, repository, prNumber)
	if err != nil {
		return domain.CommentThread{}, err
	}
	byID := make(map[int64]githubvcs.ReviewComment, len(comments))
	for _, comment := range comments {
		byID[comment.ID] = comment
	}
	rootID := resolveReviewRootID(byID, commentID)
	threadComments := make([]domain.Comment, 0, len(comments))
	var root githubvcs.ReviewComment
	if comment, ok := byID[rootID]; ok {
		root = comment
	}
	reviewSummary := githubvcs.PullRequestReviewSummary{}
	if root.ReviewID > 0 {
		if summary, err := client.GetPullRequestReview(ctx, repository, prNumber, root.ReviewID); err == nil {
			reviewSummary = summary
		}
	}
	for _, comment := range comments {
		if resolveReviewRootID(byID, comment.ID) == rootID {
			threadComments = append(threadComments, comment.ToDomain())
		}
	}
	sort.Slice(threadComments, func(i, j int) bool {
		return threadComments[i].CreatedAt.Before(threadComments[j].CreatedAt)
	})
	return domain.CommentThread{
		Kind:     domain.CommentKindReview,
		RootID:   rootID,
		Context:  buildReviewThreadContext(root, reviewSummary),
		Comments: threadComments,
	}, nil
}

func resolveReviewRootID(byID map[int64]githubvcs.ReviewComment, commentID int64) int64 {
	currentID := commentID
	for {
		comment, ok := byID[currentID]
		if !ok || comment.InReplyToID == 0 {
			return currentID
		}
		currentID = comment.InReplyToID
	}
}

func buildIssueThreadContext(prInfo githubvcs.PullRequestInfo) []string {
	title := strings.TrimSpace(prInfo.Title)
	description := strings.TrimSpace(prInfo.Description)
	if title == "" && description == "" {
		return nil
	}
	lines := []string{"PR Description:"}
	if title != "" {
		lines = append(lines, fmt.Sprintf("Title: %s", title))
	}
	if description != "" {
		lines = append(lines, description)
	}
	return lines
}

func buildReviewThreadContext(root githubvcs.ReviewComment, reviewSummary githubvcs.PullRequestReviewSummary) []string {
	lines := make([]string, 0)
	if strings.TrimSpace(root.Path) != "" {
		lines = append(lines, fmt.Sprintf("File: %s", strings.TrimSpace(root.Path)))
	}
	lineInfo := formatReviewLineInfo(root)
	if lineInfo != "" {
		lines = append(lines, lineInfo)
	}
	if strings.TrimSpace(root.DiffHunk) != "" {
		lines = append(lines, "Diff Hunk:", "```diff", root.DiffHunk, "```")
	}
	if summary := formatReviewSummary(reviewSummary); summary != "" {
		lines = append(lines, "Review Summary:", summary)
	}
	if len(lines) == 0 {
		return nil
	}
	return lines
}

func formatReviewLineInfo(root githubvcs.ReviewComment) string {
	if root.Line > 0 {
		return fmt.Sprintf("Line: %d (%s)", root.Line, strings.TrimSpace(root.Side))
	}
	if root.OriginalLine > 0 {
		return fmt.Sprintf("Original Line: %d", root.OriginalLine)
	}
	return ""
}

func formatReviewSummary(summary githubvcs.PullRequestReviewSummary) string {
	body := strings.TrimSpace(summary.Body)
	if body == "" {
		return ""
	}
	state := strings.TrimSpace(summary.State)
	author := strings.TrimSpace(summary.User.Login)
	if state != "" || author != "" {
		prefix := "Review"
		if state != "" {
			prefix = fmt.Sprintf("%s (%s)", prefix, state)
		}
		if author != "" {
			prefix = fmt.Sprintf("%s by %s", prefix, author)
		}
		return fmt.Sprintf("%s:\n%s", prefix, body)
	}
	return body
}

func isInitialPROpenedAction(action string) bool {
	return strings.EqualFold(strings.TrimSpace(action), "opened")
}

func (h *Handler) verifySignature(signatureHeader string, body []byte) bool {
	secret := strings.TrimSpace(h.webhookSecret)
	if secret == "" {
		return false
	}
	const prefix = "sha256="
	if !strings.HasPrefix(strings.ToLower(signatureHeader), prefix) {
		return false
	}
	signatureHex := strings.TrimSpace(signatureHeader[len(prefix):])
	signature, err := hex.DecodeString(signatureHex)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	if _, err := mac.Write(body); err != nil {
		return false
	}
	expected := mac.Sum(nil)
	return hmac.Equal(signature, expected)
}

func buildAuthenticatedCloneURL(rawCloneURL string, installationToken string) (string, error) {
	installationToken = strings.TrimSpace(installationToken)
	if installationToken == "" {
		return "", fmt.Errorf("installation token is required")
	}
	cloneURL, err := url.Parse(strings.TrimSpace(rawCloneURL))
	if err != nil {
		return "", err
	}
	if cloneURL.Scheme != "http" && cloneURL.Scheme != "https" {
		return "", fmt.Errorf("clone URL must use http or https")
	}
	cloneURL.User = url.UserPassword("x-access-token", installationToken)
	return cloneURL.String(), nil
}
