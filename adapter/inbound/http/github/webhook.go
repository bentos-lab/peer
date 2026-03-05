package github

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	githubvcs "bentos-backend/adapter/outbound/vcs/github"
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
	} `json:"repository"`
	PullRequest struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		Body   string `json:"body"`
		Base   struct {
			Ref string `json:"ref"`
		} `json:"base"`
		Head struct {
			Ref string `json:"ref"`
		} `json:"head"`
	} `json:"pull_request"`
}

// Handler receives GitHub webhook events and triggers review.
type Handler struct {
	reviewer      usecase.ReviewUseCase
	logger        usecase.Logger
	webhookSecret string
}

// NewHandler creates a GitHub webhook handler.
func NewHandler(reviewer usecase.ReviewUseCase, logger usecase.Logger, webhookSecret string) *Handler {
	if logger == nil {
		logger = usecase.NopLogger
	}
	return &Handler{
		reviewer:      reviewer,
		logger:        logger,
		webhookSecret: strings.TrimSpace(webhookSecret),
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

	request := usecase.ReviewRequest{
		Repository:          event.Repository.FullName,
		ChangeRequestNumber: event.PullRequest.Number,
		Title:               event.PullRequest.Title,
		Description:         event.PullRequest.Body,
		BaseRef:             event.PullRequest.Base.Ref,
		HeadRef:             event.PullRequest.Head.Ref,
		Metadata: map[string]string{
			"action": event.Action,
		},
	}
	installationID := event.Installation.ID

	go func(req usecase.ReviewRequest, action string, installationID int64) {
		startedAt := time.Now()
		h.logger.Infof("GitHub webhook background review started.")
		h.logger.Debugf("Repository is %q and change request number is %d.", req.Repository, req.ChangeRequestNumber)
		h.logger.Debugf("Webhook action is %q.", action)

		defer func() {
			if recovered := recover(); recovered != nil {
				h.logger.Errorf("GitHub webhook background review panicked.")
				h.logger.Debugf("Repository is %q, change request number is %d, and webhook action is %q.", req.Repository, req.ChangeRequestNumber, action)
				h.logger.Debugf("The background review ran for %d ms before panicking.", time.Since(startedAt).Milliseconds())
				h.logger.Debugf("Panic details: %v.", recovered)
			}
		}()

		ctx, cancel := context.WithTimeout(context.Background(), backgroundReviewTimeout)
		defer cancel()
		ctx = githubvcs.WithInstallationID(ctx, strconv.FormatInt(installationID, 10))

		if _, err := h.reviewer.Execute(ctx, req); err != nil {
			h.logger.Errorf("GitHub webhook background review failed.")
			h.logger.Debugf("Repository is %q, change request number is %d, and webhook action is %q.", req.Repository, req.ChangeRequestNumber, action)
			h.logger.Debugf("The background review ran for %d ms before failing.", time.Since(startedAt).Milliseconds())
			h.logger.Debugf("Failure details: %v.", err)
			return
		}

		h.logger.Infof("GitHub webhook background review completed.")
		h.logger.Debugf("Repository is %q, change request number is %d, and webhook action is %q.", req.Repository, req.ChangeRequestNumber, action)
		h.logger.Debugf("The background review completed in %d ms.", time.Since(startedAt).Milliseconds())
	}(request, event.Action, installationID)

	h.logger.Infof("GitHub webhook review request was accepted.")
	h.logger.Debugf("Repository is %q and change request number is %d.", request.Repository, request.ChangeRequestNumber)
	h.logger.Debugf("Webhook action is %q.", event.Action)
	w.WriteHeader(http.StatusAccepted)
}

func isValidPullRequestEvent(event pullRequestEvent) bool {
	return strings.TrimSpace(event.Repository.FullName) != "" &&
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
