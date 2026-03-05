package gitlab

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"bentos-backend/adapter/inbound/http/background"
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
	logger   usecase.Logger
}

// NewHandler creates a GitLab webhook handler.
func NewHandler(reviewer usecase.ReviewUseCase, logger usecase.Logger) *Handler {
	if logger == nil {
		logger = usecase.NopLogger
	}
	return &Handler{reviewer: reviewer, logger: logger}
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

	background.RunReviewAsync(
		h.logger,
		"GitLab",
		event.ObjectAttributes.Action,
		request,
		backgroundReviewTimeout,
		func(ctx context.Context) context.Context { return ctx },
		func(ctx context.Context, req usecase.ReviewRequest) error {
			_, err := h.reviewer.Execute(ctx, req)
			return err
		},
	)

	h.logger.Infof("GitLab webhook review request was accepted.")
	h.logger.Debugf("Repository is %q and change request number is %d.", request.Repository, request.ChangeRequestNumber)
	h.logger.Debugf("Webhook action is %q.", event.ObjectAttributes.Action)
	w.WriteHeader(http.StatusAccepted)
}

func isValidMergeRequestEvent(event mergeRequestEvent) bool {
	return strings.TrimSpace(event.Project.PathWithNamespace) != "" &&
		event.ObjectAttributes.IID > 0 &&
		strings.TrimSpace(event.ObjectAttributes.TargetBranch) != "" &&
		strings.TrimSpace(event.ObjectAttributes.SourceBranch) != ""
}
