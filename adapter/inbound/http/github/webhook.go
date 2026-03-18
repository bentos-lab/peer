package github

import (
	"context"
	"encoding/json"
	"log"
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
}

// NewHandler creates a GitHub webhook handler.
func NewHandler(reviewer usecase.ReviewUseCase) *Handler {
	return &Handler{reviewer: reviewer}
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
		ReviewID:            time.Now().UTC().Format(time.RFC3339Nano),
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
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Printf(
					"webhook background review panic provider=github repository=%q change_request_number=%d action=%q panic=%v",
					req.Repository,
					req.ChangeRequestNumber,
					action,
					recovered,
				)
			}
		}()

		ctx, cancel := context.WithTimeout(context.Background(), backgroundReviewTimeout)
		defer cancel()

		if _, err := h.reviewer.Execute(ctx, req); err != nil {
			log.Printf(
				"webhook background review failed provider=github repository=%q change_request_number=%d action=%q error=%v",
				req.Repository,
				req.ChangeRequestNumber,
				action,
				err,
			)
		}
	}(request, event.Action)

	w.WriteHeader(http.StatusAccepted)
}

func isValidPullRequestEvent(event pullRequestEvent) bool {
	return strings.TrimSpace(event.Repository.FullName) != "" &&
		event.PullRequest.Number > 0 &&
		strings.TrimSpace(event.PullRequest.Base.Ref) != "" &&
		strings.TrimSpace(event.PullRequest.Head.Ref) != ""
}
