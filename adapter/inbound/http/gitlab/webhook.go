package gitlab

import (
	"bytes"
	"context"
	"crypto/hmac"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bentos-lab/peer/adapter/inbound/http/background"
	"github.com/bentos-lab/peer/domain"
	sharedcli "github.com/bentos-lab/peer/shared/cli"
	"github.com/bentos-lab/peer/shared/jobqueue"
	"github.com/bentos-lab/peer/shared/logger/stdlogger"
	sharedlogging "github.com/bentos-lab/peer/shared/logging"
	"github.com/bentos-lab/peer/shared/text"
	"github.com/bentos-lab/peer/usecase"
	uccontracts "github.com/bentos-lab/peer/usecase/contracts"
)

const backgroundReviewTimeout = 10 * time.Minute
const backgroundOverviewTimeout = 10 * time.Minute
const backgroundReplyCommentTimeout = 10 * time.Minute
const backgroundAutogenTimeout = 10 * time.Minute

type mergeRequestEvent struct {
	ObjectKind string `json:"object_kind"`
	Project    struct {
		ID                int    `json:"id"`
		PathWithNamespace string `json:"path_with_namespace"`
	} `json:"project"`
	ObjectAttributes struct {
		IID    int    `json:"iid"`
		Action string `json:"action"`
	} `json:"object_attributes"`
}

type noteEvent struct {
	ObjectKind string `json:"object_kind"`
	User       struct {
		Username string `json:"username"`
	} `json:"user"`
	Project struct {
		ID                int    `json:"id"`
		PathWithNamespace string `json:"path_with_namespace"`
	} `json:"project"`
	ObjectAttributes struct {
		ID           int64  `json:"id"`
		Note         string `json:"note"`
		NoteableType string `json:"noteable_type"`
		Action       string `json:"action"`
	} `json:"object_attributes"`
	MergeRequest struct {
		IID int `json:"iid"`
	} `json:"merge_request"`
	Position any `json:"position"`
}

// CommentClient provides PR/comment metadata for replycomment handling.
type CommentClient interface {
	GetPullRequestInfo(ctx context.Context, repository string, pullRequestNumber int) (domain.ChangeRequestInfo, error)
	GetIssue(ctx context.Context, repository string, issueNumber int) (domain.Issue, error)
	ListIssueComments(ctx context.Context, repository string, pullRequestNumber int) ([]domain.IssueComment, error)
	ListChangeRequestComments(ctx context.Context, repository string, pullRequestNumber int) ([]domain.IssueComment, error)
	ListReviewComments(ctx context.Context, repository string, pullRequestNumber int) ([]domain.ReviewComment, error)
	GetPullRequestReview(ctx context.Context, repository string, pullRequestNumber int, reviewID int64) (domain.ReviewSummary, error)
	BuildAuthenticatedCloneURL(repository string) (string, error)
}

// RecipeConfigLoader reads enabled toggles from the repo-scoped recipe config.
type RecipeConfigLoader interface {
	Load(ctx context.Context, repoURL string, headRef string) (domain.CustomRecipe, error)
}

// Handler receives GitLab webhook events and triggers review.
type Handler struct {
	reviewBuilder       ReviewUseCaseBuilder
	overviewBuilder     OverviewUseCaseBuilder
	autogenBuilder      AutogenUseCaseBuilder
	replyCommentBuilder ReplyCommentUseCaseBuilder
	client              CommentClient
	recipeConfigLoader  RecipeConfigLoader
	codeEnvFactory      uccontracts.CodeEnvironmentFactory
	recipeLoader        usecase.CustomRecipeLoader
	logger              usecase.Logger
	webhookSecret       string
	replyTriggerName    string
	reviewEnabled       bool
	reviewEvents        []string
	reviewSuggestions   bool
	overviewEnabled     bool
	overviewEvents      []string
	overviewIssueAlign  bool
	autogenEnabled      bool
	autogenEvents       []string
	autogenDocs         bool
	autogenTests        bool
	replyEnabled        bool
	replyEvents         []string
	replyActions        []string
	jobQueue            *jobqueue.Manager
}

// ReviewUseCaseBuilder builds a review usecase for a specific repo.
type ReviewUseCaseBuilder func(repoURL string) (usecase.ReviewUseCase, error)

// OverviewUseCaseBuilder builds an overview usecase for a specific repo.
type OverviewUseCaseBuilder func(repoURL string) (usecase.OverviewUseCase, error)

// AutogenUseCaseBuilder builds an autogen usecase for a specific repo.
type AutogenUseCaseBuilder func(repoURL string) (usecase.AutogenUseCase, error)

// ReplyCommentUseCaseBuilder builds a reply comment usecase for a specific repo.
type ReplyCommentUseCaseBuilder func(repoURL string) (usecase.ReplyCommentUseCase, error)

// NewHandler creates a GitLab webhook handler.
func NewHandler(
	reviewBuilder ReviewUseCaseBuilder,
	overviewBuilder OverviewUseCaseBuilder,
	autogenBuilder AutogenUseCaseBuilder,
	replyCommentBuilder ReplyCommentUseCaseBuilder,
	client CommentClient,
	recipeConfigLoader RecipeConfigLoader,
	codeEnvFactory uccontracts.CodeEnvironmentFactory,
	recipeLoader usecase.CustomRecipeLoader,
	logger usecase.Logger,
	webhookSecret string,
	replyTriggerName string,
	reviewEnabled bool,
	reviewEvents []string,
	reviewSuggestions bool,
	overviewEnabled bool,
	overviewEvents []string,
	overviewIssueAlign bool,
	autogenEnabled bool,
	autogenEvents []string,
	autogenDocs bool,
	autogenTests bool,
	replyEnabled bool,
	replyEvents []string,
	replyActions []string,
	jobQueue *jobqueue.Manager,
) *Handler {
	if logger == nil {
		logger = stdlogger.Nop()
	}
	return &Handler{
		reviewBuilder:       reviewBuilder,
		overviewBuilder:     overviewBuilder,
		autogenBuilder:      autogenBuilder,
		replyCommentBuilder: replyCommentBuilder,
		client:              client,
		recipeConfigLoader:  recipeConfigLoader,
		codeEnvFactory:      codeEnvFactory,
		recipeLoader:        recipeLoader,
		logger:              logger,
		webhookSecret:       strings.TrimSpace(webhookSecret),
		replyTriggerName:    strings.TrimSpace(replyTriggerName),
		reviewEnabled:       reviewEnabled,
		reviewEvents:        reviewEvents,
		reviewSuggestions:   reviewSuggestions,
		overviewEnabled:     overviewEnabled,
		overviewEvents:      overviewEvents,
		overviewIssueAlign:  overviewIssueAlign,
		autogenEnabled:      autogenEnabled,
		autogenEvents:       autogenEvents,
		autogenDocs:         autogenDocs,
		autogenTests:        autogenTests,
		replyEnabled:        replyEnabled,
		replyEvents:         replyEvents,
		replyActions:        replyActions,
		jobQueue:            jobQueue,
	}
}

// ServeHTTP handles GitLab webhook events and starts review usecase.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	eventType := strings.TrimSpace(r.Header.Get("X-Gitlab-Event"))
	if eventType == "" {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	if !h.verifySignature(strings.TrimSpace(r.Header.Get("X-Gitlab-Token")), body) {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	switch strings.ToLower(eventType) {
	case "merge request hook":
		h.handleMergeRequestEvent(w, r, body)
	case "note hook":
		h.handleNoteEvent(w, r, body)
	default:
		w.WriteHeader(http.StatusAccepted)
	}
}

func (h *Handler) handleMergeRequestEvent(w http.ResponseWriter, r *http.Request, body []byte) {
	var event mergeRequestEvent
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&event); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	if !isValidMergeRequestEvent(event) {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	if err := h.ensureClient(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	repository := strings.TrimSpace(event.Project.PathWithNamespace)
	repoURL, err := h.client.BuildAuthenticatedCloneURL(repository)
	if err != nil {
		h.logWebhookRepoURLError(repository, event.ObjectAttributes.IID, err)
		http.Error(w, "invalid repository clone URL", http.StatusBadRequest)
		return
	}

	prInfo, err := h.client.GetPullRequestInfo(r.Context(), repository, event.ObjectAttributes.IID)
	if err != nil {
		http.Error(w, "failed to resolve merge request info", http.StatusBadGateway)
		return
	}

	recipeConfig := h.loadRecipeConfig(r.Context(), repoURL, prInfo.HeadRef)
	reviewEnabled := h.resolveReviewEnabled(event.ObjectAttributes.Action, recipeConfig)
	overviewEnabled := h.resolveOverviewEnabled(event.ObjectAttributes.Action, recipeConfig)
	autogenEnabled := h.resolveAutogenEnabled(event.ObjectAttributes.Action, recipeConfig)

	if !reviewEnabled {
		h.logWebhookSkipped("review", repository, event.ObjectAttributes.IID, event.ObjectAttributes.Action)
	}
	if !overviewEnabled {
		h.logWebhookSkipped("overview", repository, event.ObjectAttributes.IID, event.ObjectAttributes.Action)
	}
	if !autogenEnabled {
		h.logWebhookSkipped("autogen", repository, event.ObjectAttributes.IID, event.ObjectAttributes.Action)
	}
	if !reviewEnabled && !overviewEnabled && !autogenEnabled {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	var issueCandidates []domain.IssueContext
	if overviewEnabled && h.resolveIssueAlignmentEnabled(recipeConfig) {
		issueCandidates = resolveWebhookIssueCandidates(
			r.Context(),
			h.client,
			repository,
			prInfo.Description,
		)
	}

	input := buildWebhookInput(
		repository,
		event.ObjectAttributes.IID,
		repoURL,
		prInfo.BaseRef,
		prInfo.HeadRef,
		prInfo.Title,
		prInfo.Description,
		mapMergeRequestAction(event.ObjectAttributes.Action),
	)

	var overviewJobID string
	if overviewEnabled {
		jobID, err := h.enqueueOverviewJob(event.ObjectAttributes.Action, input, issueCandidates, prInfo.HeadRef)
		if err != nil {
			http.Error(w, "failed to enqueue overview", http.StatusInternalServerError)
			return
		}
		overviewJobID = jobID
		h.logWebhookAccepted("overview", input.Target.Repository, input.Target.ChangeRequestNumber, event.ObjectAttributes.Action)
	}

	if reviewEnabled {
		var deps []string
		if overviewJobID != "" {
			deps = []string{overviewJobID}
		}
		_, err := h.enqueueReviewJob(event.ObjectAttributes.Action, input, h.resolveReviewSuggestions(recipeConfig), prInfo.HeadRef, deps)
		if err != nil {
			http.Error(w, "failed to enqueue review", http.StatusInternalServerError)
			return
		}
		h.logWebhookAccepted("review", input.Target.Repository, input.Target.ChangeRequestNumber, event.ObjectAttributes.Action)
	}

	if autogenEnabled {
		_, err := h.enqueueAutogenJob(event.ObjectAttributes.Action, input, h.resolveAutogenDocs(recipeConfig), h.resolveAutogenTests(recipeConfig), prInfo.HeadRefName, prInfo.HeadRef)
		if err != nil {
			http.Error(w, "failed to enqueue autogen", http.StatusInternalServerError)
			return
		}
		h.logWebhookAccepted("autogen", input.Target.Repository, input.Target.ChangeRequestNumber, event.ObjectAttributes.Action)
	}

	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) handleNoteEvent(w http.ResponseWriter, r *http.Request, body []byte) {
	if h.replyCommentBuilder == nil {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	var event noteEvent
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&event); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	if !isValidNoteEvent(event) {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	if !text.ContainsTrigger(event.ObjectAttributes.Note, h.replyTriggerName) {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	if isBotAuthor(event.User.Username) {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	if err := h.ensureClient(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	repository := strings.TrimSpace(event.Project.PathWithNamespace)
	repoURL, err := h.client.BuildAuthenticatedCloneURL(repository)
	if err != nil {
		h.logWebhookRepoURLError(repository, event.MergeRequest.IID, err)
		http.Error(w, "invalid repository clone URL", http.StatusBadRequest)
		return
	}

	prInfo, err := h.client.GetPullRequestInfo(r.Context(), repository, event.MergeRequest.IID)
	if err != nil {
		http.Error(w, "failed to resolve merge request info", http.StatusBadGateway)
		return
	}
	recipeConfig := h.loadRecipeConfig(r.Context(), repoURL, prInfo.HeadRef)
	replyEnabled := h.replyEnabled
	if recipeConfig.ReplyCommentEnabled != nil {
		replyEnabled = *recipeConfig.ReplyCommentEnabled
	}
	if !replyEnabled {
		h.logWebhookSkipped("replycomment", repository, event.MergeRequest.IID, event.ObjectAttributes.Action)
		w.WriteHeader(http.StatusAccepted)
		return
	}
	if !isNoteEventAllowed(recipeConfig.ReplyCommentEvents, h.replyEvents) {
		h.logWebhookSkipped("replycomment", repository, event.MergeRequest.IID, event.ObjectAttributes.Action)
		w.WriteHeader(http.StatusAccepted)
		return
	}
	if !isActionAllowed(event.ObjectAttributes.Action, recipeConfig.ReplyCommentActions, h.replyActions) {
		h.logWebhookSkipped("replycomment", repository, event.MergeRequest.IID, event.ObjectAttributes.Action)
		w.WriteHeader(http.StatusAccepted)
		return
	}

	thread, commentKind, err := buildNoteThreadForWebhook(r.Context(), h.client, repository, event.MergeRequest.IID, event.ObjectAttributes.ID, event.Position, prInfo)
	if err != nil {
		http.Error(w, "failed to load comment thread", http.StatusBadGateway)
		return
	}

	request := usecase.ReplyCommentRequest{
		Repository:          repository,
		RepoURL:             repoURL,
		ChangeRequestNumber: event.MergeRequest.IID,
		Title:               prInfo.Title,
		Description:         prInfo.Description,
		Base:                prInfo.BaseRef,
		Head:                prInfo.HeadRef,
		CommentID:           event.ObjectAttributes.ID,
		CommentKind:         commentKind,
		Question:            text.StripTrigger(event.ObjectAttributes.Note, h.replyTriggerName),
		Thread:              thread,
		Publish:             true,
		Metadata: map[string]string{
			"action": event.ObjectAttributes.Action,
		},
	}

	background.RunReplyCommentAsync(
		h.logger,
		"GitLab",
		event.ObjectAttributes.Action,
		request,
		backgroundReplyCommentTimeout,
		func(ctx context.Context) context.Context {
			return ctx
		},
		func(ctx context.Context, req usecase.ReplyCommentRequest) error {
			if h.replyCommentBuilder == nil {
				return errors.New("reply comment usecase builder is not configured")
			}
			if h.codeEnvFactory == nil || h.recipeLoader == nil {
				return errors.New("code environment is not configured")
			}

			environment, err := h.codeEnvFactory.New(ctx, domain.CodeEnvironmentInitOptions{
				RepoURL: req.RepoURL,
			})
			if err != nil {
				return err
			}
			cleanup := environment.Cleanup
			defer func() {
				if cleanupErr := cleanup(ctx); cleanupErr != nil {
					h.logger.Warnf("Failed to cleanup code environment: %v", cleanupErr)
				}
			}()

			resolvedBase, resolvedHead, err := environment.ResolveBaseHead(ctx, req.Base, req.Head)
			if err != nil {
				return err
			}
			req.Base = resolvedBase
			req.Head = resolvedHead

			recipe, err := h.recipeLoader.Load(ctx, environment, req.Head)
			if err != nil {
				return err
			}
			req.Environment = environment
			req.Recipe = recipe

			useCase, err := h.replyCommentBuilder(req.RepoURL)
			if err != nil {
				return err
			}
			_, err = useCase.Execute(ctx, req)
			return err
		},
	)

	h.logWebhookAccepted("replycomment", request.Repository, request.ChangeRequestNumber, event.ObjectAttributes.Action)
	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) ensureClient() error {
	if h.client == nil {
		h.logger.Errorf("GitLab webhook client is not configured.")
		return errors.New("client is not configured")
	}
	return nil
}

func (h *Handler) verifySignature(signatureHeader string, body []byte) bool {
	secret := strings.TrimSpace(h.webhookSecret)
	if secret == "" {
		return false
	}
	_ = body
	given := strings.TrimSpace(signatureHeader)
	if given == "" {
		return false
	}
	return hmac.Equal([]byte(given), []byte(secret))
}

func (h *Handler) logWebhookRepoURLError(repository string, prNumber int, err error) {
	h.logger.Errorf("GitLab webhook failed to build repository clone URL.")
	h.logger.Debugf("Repository is %q and change request number is %d.", repository, prNumber)
	h.logger.Debugf("Failure details: %v.", err)
}

func (h *Handler) logWebhookAccepted(kind string, repository string, prNumber int, action string) {
	h.logger.Infof("GitLab webhook %s request was accepted.", kind)
	h.logger.Debugf("Repository is %q and change request number is %d.", repository, prNumber)
	h.logger.Debugf("Webhook action is %q.", action)
}

func (h *Handler) logWebhookSkipped(kind string, repository string, prNumber int, action string) {
	h.logger.Infof("GitLab webhook %s request was skipped.", kind)
	h.logger.Debugf("Repository is %q and change request number is %d.", repository, prNumber)
	h.logger.Debugf("Webhook action is %q.", action)
}

func (h *Handler) loadRecipeConfig(ctx context.Context, repoURL string, headRef string) domain.CustomRecipe {
	if h.recipeConfigLoader == nil {
		return domain.CustomRecipe{}
	}
	recipe, err := h.recipeConfigLoader.Load(ctx, repoURL, headRef)
	if err != nil {
		h.logger.Warnf("Failed to load webhook recipe config: %v", err)
		return domain.CustomRecipe{}
	}
	return recipe
}

func (h *Handler) enqueueReviewJob(action string, input domain.ChangeRequestInput, suggestions bool, headRef string, deps []string) (string, error) {
	if h.jobQueue == nil {
		return "", errors.New("job queue is not configured")
	}
	return h.jobQueue.Enqueue(jobqueue.Job{
		Name:      "review",
		DependsOn: deps,
		Run: func() error {
			startedAt := time.Now()
			defer func() {
				if recovered := recover(); recovered != nil {
					h.logger.Errorf(
						"GitLab webhook review panicked for %q#%d action=%q after %d ms: %v.",
						input.Target.Repository,
						input.Target.ChangeRequestNumber,
						action,
						time.Since(startedAt).Milliseconds(),
						recovered,
					)
				}
			}()
			ctx, cancel := context.WithTimeout(context.Background(), backgroundReviewTimeout)
			defer cancel()
			if h.reviewBuilder == nil {
				return errors.New("review usecase builder is not configured")
			}
			if h.codeEnvFactory == nil || h.recipeLoader == nil {
				return errors.New("code environment is not configured")
			}
			environment, err := h.codeEnvFactory.New(ctx, domain.CodeEnvironmentInitOptions{
				RepoURL: input.RepoURL,
			})
			if err != nil {
				return err
			}
			cleanup := environment.Cleanup
			defer func() {
				if cleanupErr := cleanup(ctx); cleanupErr != nil {
					h.logger.Warnf("Failed to cleanup code environment: %v", cleanupErr)
				}
			}()

			resolvedBase, resolvedHead, err := environment.ResolveBaseHead(ctx, input.Base, input.Head)
			if err != nil {
				return err
			}
			input.Base = resolvedBase
			input.Head = resolvedHead
			headRef = resolvedHead

			recipe, err := h.recipeLoader.Load(ctx, environment, headRef)
			if err != nil {
				return err
			}

			request := usecase.ReviewRequest{
				Input:       input,
				Suggestions: suggestions,
				Environment: environment,
				Recipe:      recipe,
			}
			sharedlogging.LogInputSnapshot(h.logger, "webhook", action, request)

			useCase, err := h.reviewBuilder(input.RepoURL)
			if err != nil {
				return err
			}
			_, err = useCase.Execute(ctx, request)
			if err != nil {
				h.logger.Debugf(
					"GitLab webhook review failed for %q#%d action=%q after %d ms.",
					input.Target.Repository,
					input.Target.ChangeRequestNumber,
					action,
					time.Since(startedAt).Milliseconds(),
				)
			}
			return err
		},
	})
}

func (h *Handler) enqueueOverviewJob(action string, input domain.ChangeRequestInput, issueCandidates []domain.IssueContext, headRef string) (string, error) {
	if h.jobQueue == nil {
		return "", errors.New("job queue is not configured")
	}
	return h.jobQueue.Enqueue(jobqueue.Job{
		Name: "overview",
		Run: func() error {
			startedAt := time.Now()
			defer func() {
				if recovered := recover(); recovered != nil {
					h.logger.Errorf(
						"GitLab webhook overview panicked for %q#%d action=%q after %d ms: %v.",
						input.Target.Repository,
						input.Target.ChangeRequestNumber,
						action,
						time.Since(startedAt).Milliseconds(),
						recovered,
					)
				}
			}()
			ctx, cancel := context.WithTimeout(context.Background(), backgroundOverviewTimeout)
			defer cancel()
			if h.overviewBuilder == nil {
				return errors.New("overview usecase builder is not configured")
			}
			if h.codeEnvFactory == nil || h.recipeLoader == nil {
				return errors.New("code environment is not configured")
			}

			environment, err := h.codeEnvFactory.New(ctx, domain.CodeEnvironmentInitOptions{
				RepoURL: input.RepoURL,
			})
			if err != nil {
				return err
			}
			cleanup := environment.Cleanup
			defer func() {
				if cleanupErr := cleanup(ctx); cleanupErr != nil {
					h.logger.Warnf("Failed to cleanup code environment: %v", cleanupErr)
				}
			}()

			resolvedBase, resolvedHead, err := environment.ResolveBaseHead(ctx, input.Base, input.Head)
			if err != nil {
				return err
			}
			input.Base = resolvedBase
			input.Head = resolvedHead
			headRef = resolvedHead

			recipe, err := h.recipeLoader.Load(ctx, environment, headRef)
			if err != nil {
				return err
			}

			request := usecase.OverviewRequest{
				Input:          input,
				IssueAlignment: usecase.OverviewIssueAlignmentInput{Candidates: issueCandidates},
				Environment:    environment,
				Recipe:         recipe,
			}
			sharedlogging.LogInputSnapshot(h.logger, "webhook", action, request)

			useCase, err := h.overviewBuilder(input.RepoURL)
			if err != nil {
				return err
			}
			_, err = useCase.Execute(ctx, request)
			if err != nil {
				h.logger.Debugf(
					"GitLab webhook overview failed for %q#%d action=%q after %d ms.",
					input.Target.Repository,
					input.Target.ChangeRequestNumber,
					action,
					time.Since(startedAt).Milliseconds(),
				)
			}
			return err
		},
	})
}

func (h *Handler) enqueueAutogenJob(action string, input domain.ChangeRequestInput, docs bool, tests bool, headBranch string, headRef string) (string, error) {
	if h.jobQueue == nil {
		return "", errors.New("job queue is not configured")
	}
	return h.jobQueue.Enqueue(jobqueue.Job{
		Name: "autogen",
		Run: func() error {
			startedAt := time.Now()
			defer func() {
				if recovered := recover(); recovered != nil {
					h.logger.Errorf(
						"GitLab webhook autogen panicked for %q#%d action=%q after %d ms: %v.",
						input.Target.Repository,
						input.Target.ChangeRequestNumber,
						action,
						time.Since(startedAt).Milliseconds(),
						recovered,
					)
				}
			}()
			ctx, cancel := context.WithTimeout(context.Background(), backgroundAutogenTimeout)
			defer cancel()
			if h.autogenBuilder == nil {
				return errors.New("autogen usecase builder is not configured")
			}
			if h.codeEnvFactory == nil || h.recipeLoader == nil {
				return errors.New("code environment is not configured")
			}

			environment, err := h.codeEnvFactory.New(ctx, domain.CodeEnvironmentInitOptions{
				RepoURL: input.RepoURL,
			})
			if err != nil {
				return err
			}
			cleanup := environment.Cleanup
			defer func() {
				if cleanupErr := cleanup(ctx); cleanupErr != nil {
					h.logger.Warnf("Failed to cleanup code environment: %v", cleanupErr)
				}
			}()

			resolvedBase, resolvedHead, err := environment.ResolveBaseHead(ctx, input.Base, input.Head)
			if err != nil {
				return err
			}
			input.Base = resolvedBase
			input.Head = resolvedHead
			headRef = resolvedHead

			recipe, err := h.recipeLoader.Load(ctx, environment, headRef)
			if err != nil {
				return err
			}

			request := usecase.AutogenRequest{
				Input:       input,
				Docs:        docs,
				Tests:       tests,
				Publish:     true,
				HeadBranch:  headBranch,
				Environment: environment,
				Recipe:      recipe,
			}
			sharedlogging.LogInputSnapshot(h.logger, "webhook", action, request)

			useCase, err := h.autogenBuilder(input.RepoURL)
			if err != nil {
				return err
			}
			_, err = useCase.Execute(ctx, request)
			if err != nil {
				h.logger.Debugf(
					"GitLab webhook autogen failed for %q#%d action=%q after %d ms.",
					input.Target.Repository,
					input.Target.ChangeRequestNumber,
					action,
					time.Since(startedAt).Milliseconds(),
				)
			}
			return err
		},
	})
}

func (h *Handler) resolveReviewEnabled(action string, recipe domain.CustomRecipe) bool {
	reviewEnabled := h.reviewEnabled
	if recipe.ReviewEnabled != nil {
		reviewEnabled = *recipe.ReviewEnabled
	}
	return reviewEnabled && isActionAllowed(mapMergeRequestAction(action), recipe.ReviewEvents, h.reviewEvents)
}

func (h *Handler) resolveOverviewEnabled(action string, recipe domain.CustomRecipe) bool {
	overviewEnabled := h.overviewEnabled
	if recipe.OverviewEnabled != nil {
		overviewEnabled = *recipe.OverviewEnabled
	}
	return overviewEnabled && isActionAllowed(mapMergeRequestAction(action), recipe.OverviewEvents, h.overviewEvents)
}

func (h *Handler) resolveAutogenEnabled(action string, recipe domain.CustomRecipe) bool {
	autogenEnabled := h.autogenEnabled
	if recipe.AutogenEnabled != nil {
		autogenEnabled = *recipe.AutogenEnabled
	}
	autogenEnabled = autogenEnabled && isActionAllowed(mapMergeRequestAction(action), recipe.AutogenEvents, h.autogenEvents)
	if !h.resolveAutogenDocs(recipe) && !h.resolveAutogenTests(recipe) {
		return false
	}
	return autogenEnabled
}

func (h *Handler) resolveReviewSuggestions(recipe domain.CustomRecipe) bool {
	return sharedcli.ResolveBool(recipe.ReviewSuggestions, nil, h.reviewSuggestions)
}

func (h *Handler) resolveIssueAlignmentEnabled(recipe domain.CustomRecipe) bool {
	return sharedcli.ResolveBool(recipe.OverviewIssueAlignmentEnabled, nil, h.overviewIssueAlign)
}

func (h *Handler) resolveAutogenDocs(recipe domain.CustomRecipe) bool {
	return sharedcli.ResolveBool(recipe.AutogenDocs, nil, h.autogenDocs)
}

func (h *Handler) resolveAutogenTests(recipe domain.CustomRecipe) bool {
	return sharedcli.ResolveBool(recipe.AutogenTests, nil, h.autogenTests)
}

func buildWebhookInput(repository string, prNumber int, repoURL string, base string, head string, title string, description string, action string) domain.ChangeRequestInput {
	return domain.ChangeRequestInput{
		Target:      domain.ChangeRequestTarget{Repository: repository, ChangeRequestNumber: prNumber},
		RepoURL:     repoURL,
		Base:        base,
		Head:        head,
		Title:       title,
		Description: description,
		Language:    "English",
		Metadata: map[string]string{
			"action": action,
		},
	}
}

func mapMergeRequestAction(action string) string {
	normalized := strings.ToLower(strings.TrimSpace(action))
	switch normalized {
	case "open":
		return "opened"
	case "reopen":
		return "reopened"
	case "update":
		return "synchronize"
	default:
		return normalized
	}
}

func resolveWebhookIssueCandidates(
	ctx context.Context,
	client CommentClient,
	repository string,
	description string,
) []domain.IssueContext {
	references := text.ExtractGitLabIssueReferences(description, repository)
	if len(references) == 0 {
		return nil
	}

	candidates := make([]domain.IssueContext, 0, len(references))
	for _, ref := range references {
		issue, err := client.GetIssue(ctx, ref.Repository, ref.Number)
		if err != nil {
			continue
		}
		comments, err := client.ListIssueComments(ctx, ref.Repository, ref.Number)
		if err != nil {
			continue
		}
		issueComments := make([]domain.Comment, 0, len(comments))
		for _, comment := range comments {
			issueComments = append(issueComments, comment.ToDomain())
		}
		candidates = append(candidates, domain.IssueContext{
			Issue:    issue,
			Comments: issueComments,
		})
	}
	if len(candidates) == 0 {
		return nil
	}
	return candidates
}

func isValidMergeRequestEvent(event mergeRequestEvent) bool {
	return strings.EqualFold(strings.TrimSpace(event.ObjectKind), "merge_request") &&
		strings.TrimSpace(event.Project.PathWithNamespace) != "" &&
		event.ObjectAttributes.IID > 0
}

func isValidNoteEvent(event noteEvent) bool {
	return strings.EqualFold(strings.TrimSpace(event.ObjectKind), "note") &&
		strings.EqualFold(strings.TrimSpace(event.ObjectAttributes.NoteableType), "MergeRequest") &&
		strings.TrimSpace(event.Project.PathWithNamespace) != "" &&
		event.ObjectAttributes.ID > 0 &&
		event.MergeRequest.IID > 0
}

func isBotAuthor(username string) bool {
	return strings.Contains(strings.ToLower(strings.TrimSpace(username)), "bot")
}

func buildNoteThreadForWebhook(
	ctx context.Context,
	client CommentClient,
	repository string,
	prNumber int,
	commentID int64,
	position any,
	prInfo domain.ChangeRequestInfo,
) (domain.CommentThread, domain.CommentKindEnum, error) {
	if position == nil {
		comments, err := client.ListChangeRequestComments(ctx, repository, prNumber)
		if err != nil {
			return domain.CommentThread{}, domain.CommentKindIssue, err
		}
		threadComments := make([]domain.Comment, 0, len(comments))
		for _, comment := range comments {
			threadComments = append(threadComments, comment.ToDomain())
		}
		return domain.CommentThread{
			Kind:     domain.CommentKindIssue,
			RootID:   commentID,
			Context:  buildIssueThreadContext(prInfo),
			Comments: threadComments,
		}, domain.CommentKindIssue, nil
	}

	comments, err := client.ListReviewComments(ctx, repository, prNumber)
	if err != nil {
		return domain.CommentThread{}, domain.CommentKindReview, err
	}
	byID := make(map[int64]domain.ReviewComment, len(comments))
	for _, comment := range comments {
		byID[comment.ID] = comment
	}
	rootID := resolveReviewRootID(byID, commentID)
	threadComments := make([]domain.Comment, 0, len(comments))
	var root domain.ReviewComment
	if comment, ok := byID[rootID]; ok {
		root = comment
	}
	reviewSummary := domain.ReviewSummary{}
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
	return domain.CommentThread{
		Kind:     domain.CommentKindReview,
		RootID:   rootID,
		Context:  buildReviewThreadContext(root, reviewSummary),
		Comments: threadComments,
	}, domain.CommentKindReview, nil
}

func resolveReviewRootID(byID map[int64]domain.ReviewComment, commentID int64) int64 {
	currentID := commentID
	for {
		comment, ok := byID[currentID]
		if !ok || comment.InReplyToID == 0 {
			return currentID
		}
		currentID = comment.InReplyToID
	}
}

func buildIssueThreadContext(prInfo domain.ChangeRequestInfo) []string {
	title := strings.TrimSpace(prInfo.Title)
	description := strings.TrimSpace(prInfo.Description)
	if title == "" && description == "" {
		return nil
	}
	lines := []string{"MR Description:"}
	if title != "" {
		lines = append(lines, "Title: "+title)
	}
	if description != "" {
		lines = append(lines, description)
	}
	return lines
}

func buildReviewThreadContext(root domain.ReviewComment, reviewSummary domain.ReviewSummary) []string {
	lines := make([]string, 0)
	if strings.TrimSpace(root.Path) != "" {
		lines = append(lines, "File: "+strings.TrimSpace(root.Path))
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

func formatReviewLineInfo(root domain.ReviewComment) string {
	if root.Line > 0 {
		return "Line: " + strconv.Itoa(root.Line) + " (" + strings.TrimSpace(root.Side) + ")"
	}
	if root.OriginalLine > 0 {
		return "Original Line: " + strconv.Itoa(root.OriginalLine)
	}
	return ""
}

func formatReviewSummary(summary domain.ReviewSummary) string {
	body := strings.TrimSpace(summary.Body)
	if body == "" {
		return ""
	}
	state := strings.TrimSpace(summary.State)
	author := strings.TrimSpace(summary.User.Login)
	if state != "" || author != "" {
		prefix := "Review"
		if state != "" {
			prefix = prefix + " (" + state + ")"
		}
		if author != "" {
			prefix = prefix + " by " + author
		}
		return prefix + ":\n" + body
	}
	return body
}

func isActionAllowed(value string, allowlist []string, defaultAllowlist []string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return false
	}
	if allowlist == nil {
		return containsNormalized(defaultAllowlist, normalized)
	}
	return containsNormalized(allowlist, normalized)
}

func containsNormalized(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), target) {
			return true
		}
	}
	return false
}

func isNoteEventAllowed(allowlist []string, defaultAllowlist []string) bool {
	effective := allowlist
	if allowlist == nil {
		effective = defaultAllowlist
	}
	return containsNormalized(effective, "note") || containsNormalized(effective, "issue_comment")
}
