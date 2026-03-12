package github

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	githubvcs "bentos-backend/adapter/outbound/vcs/github"
	"bentos-backend/shared/logger/stdlogger"
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
	changeRequestBuilder ChangeRequestUseCaseBuilder
	replyCommentBuilder  ReplyCommentUseCaseBuilder
	tokenProvider        CommentClient
	logger               usecase.Logger
	webhookSecret        string
	replyTriggerName     string
	enableOverview       bool
	enableSuggestions    bool
}

// ChangeRequestUseCaseBuilder builds a change request usecase for a specific repo.
type ChangeRequestUseCaseBuilder func(repoURL string) (usecase.ChangeRequestUseCase, error)

// ReplyCommentUseCaseBuilder builds a reply comment usecase for a specific repo.
type ReplyCommentUseCaseBuilder func(repoURL string) (usecase.ReplyCommentUseCase, error)

// NewHandler creates a GitHub webhook handler.
func NewHandler(
	changeRequestBuilder ChangeRequestUseCaseBuilder,
	replyCommentBuilder ReplyCommentUseCaseBuilder,
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
		changeRequestBuilder: changeRequestBuilder,
		replyCommentBuilder:  replyCommentBuilder,
		tokenProvider:        tokenProvider,
		logger:               logger,
		webhookSecret:        strings.TrimSpace(webhookSecret),
		replyTriggerName:     strings.TrimSpace(replyTriggerName),
		enableOverview:       enableOverview,
		enableSuggestions:    enableSuggestions,
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
