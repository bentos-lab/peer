package github

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"bentos-backend/usecase"
)

const backgroundReviewTimeout = 10 * time.Minute

type pullRequestEvent struct {
	Action     string `json:"action"`
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
	reviewer usecase.ReviewUseCase
	logger   usecase.Logger
}

// NewHandler creates a GitHub webhook handler.
func NewHandler(reviewer usecase.ReviewUseCase, logger usecase.Logger) *Handler {
	if logger == nil {
		logger = usecase.NopLogger
	}
	return &Handler{reviewer: reviewer, logger: logger}
}

// ServeHTTP handles pull_request events and starts review usecase.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var event pullRequestEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	if !isValidPullRequestEvent(event) {
		http.Error(w, "invalid payload", http.StatusBadRequest)
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

	go func(req usecase.ReviewRequest, action string) {
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
	}(request, event.Action)

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
