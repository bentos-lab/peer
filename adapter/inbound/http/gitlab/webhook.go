package gitlab

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

type mergeRequestEvent struct {
	ObjectKind string `json:"object_kind"`
	Project    struct {
		PathWithNamespace string `json:"path_with_namespace"`
	} `json:"project"`
	ObjectAttributes struct {
		IID          int    `json:"iid"`
		Title        string `json:"title"`
		Description  string `json:"description"`
		TargetBranch string `json:"target_branch"`
		SourceBranch string `json:"source_branch"`
		Action       string `json:"action"`
	} `json:"object_attributes"`
}

// Handler receives GitLab webhook events and triggers review.
type Handler struct {
	reviewer usecase.ReviewUseCase
}

// NewHandler creates a GitLab webhook handler.
func NewHandler(reviewer usecase.ReviewUseCase) *Handler {
	return &Handler{reviewer: reviewer}
}

// ServeHTTP handles merge request events and starts review usecase.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var event mergeRequestEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	if !isValidMergeRequestEvent(event) {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	request := usecase.ReviewRequest{
		ReviewID:            time.Now().UTC().Format(time.RFC3339Nano),
		Repository:          event.Project.PathWithNamespace,
		ChangeRequestNumber: event.ObjectAttributes.IID,
		Title:               event.ObjectAttributes.Title,
		Description:         event.ObjectAttributes.Description,
		BaseRef:             event.ObjectAttributes.TargetBranch,
		HeadRef:             event.ObjectAttributes.SourceBranch,
		Metadata: map[string]string{
			"action": event.ObjectAttributes.Action,
		},
	}

	go func(req usecase.ReviewRequest, action string) {
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Printf(
					"webhook background review panic provider=gitlab repository=%q change_request_number=%d action=%q panic=%v",
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
				"webhook background review failed provider=gitlab repository=%q change_request_number=%d action=%q error=%v",
				req.Repository,
				req.ChangeRequestNumber,
				action,
				err,
			)
		}
	}(request, event.ObjectAttributes.Action)

	w.WriteHeader(http.StatusAccepted)
}

func isValidMergeRequestEvent(event mergeRequestEvent) bool {
	return strings.TrimSpace(event.Project.PathWithNamespace) != "" &&
		event.ObjectAttributes.IID > 0 &&
		strings.TrimSpace(event.ObjectAttributes.TargetBranch) != "" &&
		strings.TrimSpace(event.ObjectAttributes.SourceBranch) != ""
}
