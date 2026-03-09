package github

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"bentos-backend/adapter/inbound/http/background"
	githubvcs "bentos-backend/adapter/outbound/vcs/github"
	"bentos-backend/shared/logger/stdlogger"
	"bentos-backend/usecase"
)

const backgroundReviewTimeout = 10 * time.Minute

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

// InstallationTokenProvider resolves installation access tokens.
type InstallationTokenProvider interface {
	GetInstallationAccessToken(ctx context.Context, installationID string) (string, error)
}

// Handler receives GitHub webhook events and triggers review.
type Handler struct {
	reviewer          usecase.ChangeRequestUseCase
	tokenProvider     InstallationTokenProvider
	logger            usecase.Logger
	webhookSecret     string
	enableOverview    bool
	enableSuggestions bool
}

// NewHandler creates a GitHub webhook handler.
func NewHandler(
	changeRequestUseCase usecase.ChangeRequestUseCase,
	tokenProvider InstallationTokenProvider,
	logger usecase.Logger,
	webhookSecret string,
	enableOverview bool,
	enableSuggestions bool,
) *Handler {
	if logger == nil {
		logger = stdlogger.Nop()
	}
	return &Handler{
		reviewer:          changeRequestUseCase,
		tokenProvider:     tokenProvider,
		logger:            logger,
		webhookSecret:     strings.TrimSpace(webhookSecret),
		enableOverview:    enableOverview,
		enableSuggestions: enableSuggestions,
	}
}

// ServeHTTP handles pull_request events and starts review usecase.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !strings.EqualFold(strings.TrimSpace(r.Header.Get("X-GitHub-Event")), "pull_request") {
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
	if event.Installation.ID <= 0 {
		h.logger.Errorf("GitHub webhook payload is missing installation id.")
		h.logger.Debugf("Repository is %q and change request number is %d.", event.Repository.FullName, event.PullRequest.Number)
		http.Error(w, "missing installation id", http.StatusBadRequest)
		return
	}
	if h.tokenProvider == nil {
		h.logger.Errorf("GitHub webhook token provider is not configured.")
		http.Error(w, "token provider is not configured", http.StatusInternalServerError)
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
		Provider:            "github",
		Repository:          event.Repository.FullName,
		RepoURL:             repoURL,
		ChangeRequestNumber: event.PullRequest.Number,
		Title:               event.PullRequest.Title,
		Description:         event.PullRequest.Body,
		Base:                base,
		Head:                head,
		Comment:             true,
		EnableOverview:      h.enableOverview && isInitialPROpenedAction(event.Action),
		EnableSuggestions:   h.enableSuggestions,
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
