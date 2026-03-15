package github

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"bentos-backend/adapter/inbound/cli"
	"bentos-backend/adapter/inbound/http/background"
	codeenv "bentos-backend/adapter/outbound/codeenv"
	githubvcs "bentos-backend/adapter/outbound/vcs/github"
	"bentos-backend/domain"
	"bentos-backend/shared/logger/stdlogger"
	"bentos-backend/shared/text"
	"bentos-backend/usecase"
	uccontracts "bentos-backend/usecase/contracts"
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
	GetIssue(ctx context.Context, repository string, issueNumber int) (githubvcs.Issue, error)
	GetPullRequestReview(ctx context.Context, repository string, pullRequestNumber int, reviewID int64) (githubvcs.PullRequestReviewSummary, error)
	ListIssueComments(ctx context.Context, repository string, pullRequestNumber int) ([]githubvcs.IssueComment, error)
	ListReviewComments(ctx context.Context, repository string, pullRequestNumber int) ([]githubvcs.ReviewComment, error)
}

// RecipeConfigLoader reads enabled toggles from the repo-scoped recipe config.
type RecipeConfigLoader interface {
	Load(ctx context.Context, repoURL string, headRef string) (domain.CustomRecipe, error)
}

// Handler receives GitHub webhook events and triggers review.
type Handler struct {
	changeRequestBuilder ChangeRequestUseCaseBuilder
	replyCommentBuilder  ReplyCommentUseCaseBuilder
	tokenProvider        CommentClient
	recipeConfigLoader   RecipeConfigLoader
	codeEnvFactory       uccontracts.CodeEnvironmentFactory
	recipeLoader         usecase.CustomRecipeLoader
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
	recipeConfigLoader RecipeConfigLoader,
	codeEnvFactory uccontracts.CodeEnvironmentFactory,
	recipeLoader usecase.CustomRecipeLoader,
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
		recipeConfigLoader:   recipeConfigLoader,
		codeEnvFactory:       codeEnvFactory,
		recipeLoader:         recipeLoader,
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
	if err := h.ensureInstallation(event.Installation.ID, event.Repository.FullName, event.PullRequest.Number); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	installationID := strconv.FormatInt(event.Installation.ID, 10)
	installationToken, err := h.tokenProvider.GetInstallationAccessToken(r.Context(), installationID)
	if err != nil {
		h.logWebhookTokenError(event.Repository.FullName, event.PullRequest.Number, err)
		http.Error(w, "failed to resolve installation token", http.StatusBadGateway)
		return
	}
	repoURL, err := buildAuthenticatedCloneURL(event.Repository.CloneURL, installationToken)
	if err != nil {
		h.logWebhookRepoURLError(event.Repository.FullName, event.PullRequest.Number, err)
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

	recipeConfig := h.loadRecipeConfig(r.Context(), repoURL, head)
	if !isActionAllowed(event.Action, recipeConfig.ReviewEvents, defaultReviewActions) {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	if recipeConfig.ReviewEnabled != nil && !*recipeConfig.ReviewEnabled {
		h.logWebhookSkipped("review", event.Repository.FullName, event.PullRequest.Number, event.Action)
		w.WriteHeader(http.StatusAccepted)
		return
	}

	enableOverview := h.enableOverview && isActionAllowed(event.Action, recipeConfig.OverviewEvents, defaultOverviewActions)
	if recipeConfig.OverviewEnabled != nil && !*recipeConfig.OverviewEnabled {
		enableOverview = false
	}
	enableSuggestions := cli.ResolveBool(recipeConfig.ReviewSuggestions, nil, h.enableSuggestions)
	issueAlignmentEnabled := cli.ResolveBool(recipeConfig.OverviewIssueAlignmentEnabled, nil, true)

	var issueCandidates []domain.IssueContext
	if issueAlignmentEnabled {
		issueCandidates = resolveWebhookIssueCandidates(
			githubvcs.WithInstallationID(r.Context(), installationID),
			h.tokenProvider,
			event.Repository.FullName,
			event.PullRequest.Body,
		)
	}

	request := usecase.ChangeRequestRequest{
		Repository:          event.Repository.FullName,
		RepoURL:             repoURL,
		ChangeRequestNumber: event.PullRequest.Number,
		Title:               event.PullRequest.Title,
		Description:         event.PullRequest.Body,
		Base:                base,
		Head:                head,
		EnableReview:        true,
		EnableOverview:      enableOverview,
		EnableSuggestions:   enableSuggestions,
		OverviewIssueAlignment: usecase.OverviewIssueAlignmentInput{
			Candidates: issueCandidates,
		},
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
			if h.changeRequestBuilder == nil {
				return errors.New("change request usecase builder is not configured")
			}
			if h.codeEnvFactory == nil || h.recipeLoader == nil {
				return errors.New("code environment is not configured")
			}

			environment, cleanup, err := codeenv.NewEnvironment(ctx, h.codeEnvFactory, req.RepoURL)
			if err != nil {
				return err
			}
			defer func() {
				if cleanupErr := cleanup(ctx); cleanupErr != nil {
					h.logger.Warnf("Failed to cleanup code environment: %v", cleanupErr)
				}
			}()

			recipe, err := h.recipeLoader.Load(ctx, environment, req.Head)
			if err != nil {
				return err
			}
			req.Environment = environment
			req.Recipe = recipe

			useCase, err := h.changeRequestBuilder(req.RepoURL)
			if err != nil {
				return err
			}
			_, err = useCase.Execute(ctx, req)
			return err
		},
	)

	h.logWebhookAccepted("review", request.Repository, request.ChangeRequestNumber, event.Action)
	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) handleIssueCommentEvent(w http.ResponseWriter, r *http.Request, body []byte) {
	if h.replyCommentBuilder == nil {
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
		h.logWebhookTokenError(event.Repository.FullName, event.Issue.Number, err)
		http.Error(w, "failed to resolve installation token", http.StatusBadGateway)
		return
	}
	repoURL, err := buildAuthenticatedCloneURL(event.Repository.CloneURL, installationToken)
	if err != nil {
		h.logWebhookRepoURLError(event.Repository.FullName, event.Issue.Number, err)
		http.Error(w, "invalid repository clone URL", http.StatusBadRequest)
		return
	}

	ctx := githubvcs.WithInstallationID(r.Context(), installationID)
	prInfo, err := h.tokenProvider.GetPullRequestInfo(ctx, event.Repository.FullName, event.Issue.Number)
	if err != nil {
		http.Error(w, "failed to resolve pull request info", http.StatusBadGateway)
		return
	}
	recipeConfig := h.loadRecipeConfig(ctx, repoURL, prInfo.HeadRef)
	if !isActionAllowed("issue_comment", recipeConfig.AutoreplyEvents, defaultAutoreplyEvents) {
		h.logWebhookSkipped("replycomment", event.Repository.FullName, event.Issue.Number, event.Action)
		w.WriteHeader(http.StatusAccepted)
		return
	}
	if !isActionAllowed(event.Action, recipeConfig.AutoreplyActions, defaultAutoreplyActions) {
		h.logWebhookSkipped("replycomment", event.Repository.FullName, event.Issue.Number, event.Action)
		w.WriteHeader(http.StatusAccepted)
		return
	}
	if recipeConfig.AutoreplyEnabled != nil && !*recipeConfig.AutoreplyEnabled {
		h.logWebhookSkipped("replycomment", event.Repository.FullName, event.Issue.Number, event.Action)
		w.WriteHeader(http.StatusAccepted)
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
			if h.replyCommentBuilder == nil {
				return errors.New("reply comment usecase builder is not configured")
			}
			if h.codeEnvFactory == nil || h.recipeLoader == nil {
				return errors.New("code environment is not configured")
			}

			environment, cleanup, err := codeenv.NewEnvironment(ctx, h.codeEnvFactory, req.RepoURL)
			if err != nil {
				return err
			}
			defer func() {
				if cleanupErr := cleanup(ctx); cleanupErr != nil {
					h.logger.Warnf("Failed to cleanup code environment: %v", cleanupErr)
				}
			}()

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

	h.logWebhookAccepted("replycomment", request.Repository, request.ChangeRequestNumber, event.Action)
	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) handleReviewCommentEvent(w http.ResponseWriter, r *http.Request, body []byte) {
	if h.replyCommentBuilder == nil {
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
		h.logWebhookTokenError(event.Repository.FullName, event.PullRequest.Number, err)
		http.Error(w, "failed to resolve installation token", http.StatusBadGateway)
		return
	}
	repoURL, err := buildAuthenticatedCloneURL(event.Repository.CloneURL, installationToken)
	if err != nil {
		h.logWebhookRepoURLError(event.Repository.FullName, event.PullRequest.Number, err)
		http.Error(w, "invalid repository clone URL", http.StatusBadRequest)
		return
	}

	ctx := githubvcs.WithInstallationID(r.Context(), installationID)
	prInfo, err := h.tokenProvider.GetPullRequestInfo(ctx, event.Repository.FullName, event.PullRequest.Number)
	if err != nil {
		http.Error(w, "failed to resolve pull request info", http.StatusBadGateway)
		return
	}
	recipeConfig := h.loadRecipeConfig(ctx, repoURL, prInfo.HeadRef)
	if !isActionAllowed("pull_request_review_comment", recipeConfig.AutoreplyEvents, defaultAutoreplyEvents) {
		h.logWebhookSkipped("replycomment", event.Repository.FullName, event.PullRequest.Number, event.Action)
		w.WriteHeader(http.StatusAccepted)
		return
	}
	if !isActionAllowed(event.Action, recipeConfig.AutoreplyActions, defaultAutoreplyActions) {
		h.logWebhookSkipped("replycomment", event.Repository.FullName, event.PullRequest.Number, event.Action)
		w.WriteHeader(http.StatusAccepted)
		return
	}
	if recipeConfig.AutoreplyEnabled != nil && !*recipeConfig.AutoreplyEnabled {
		h.logWebhookSkipped("replycomment", event.Repository.FullName, event.PullRequest.Number, event.Action)
		w.WriteHeader(http.StatusAccepted)
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
			if h.replyCommentBuilder == nil {
				return errors.New("reply comment usecase builder is not configured")
			}
			if h.codeEnvFactory == nil || h.recipeLoader == nil {
				return errors.New("code environment is not configured")
			}

			environment, cleanup, err := codeenv.NewEnvironment(ctx, h.codeEnvFactory, req.RepoURL)
			if err != nil {
				return err
			}
			defer func() {
				if cleanupErr := cleanup(ctx); cleanupErr != nil {
					h.logger.Warnf("Failed to cleanup code environment: %v", cleanupErr)
				}
			}()

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

	h.logWebhookAccepted("replycomment", request.Repository, request.ChangeRequestNumber, event.Action)
	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) logWebhookTokenError(repository string, prNumber int, err error) {
	h.logger.Errorf("GitHub webhook failed to resolve installation token.")
	h.logger.Debugf("Repository is %q and change request number is %d.", repository, prNumber)
	h.logger.Debugf("Failure details: %v.", err)
}

func (h *Handler) logWebhookRepoURLError(repository string, prNumber int, err error) {
	h.logger.Errorf("GitHub webhook failed to build repository clone URL.")
	h.logger.Debugf("Repository is %q and change request number is %d.", repository, prNumber)
	h.logger.Debugf("Failure details: %v.", err)
}

func (h *Handler) logWebhookAccepted(kind string, repository string, prNumber int, action string) {
	h.logger.Infof("GitHub webhook %s request was accepted.", kind)
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

func (h *Handler) logWebhookSkipped(kind string, repository string, prNumber int, action string) {
	h.logger.Infof("GitHub webhook %s request was skipped.", kind)
	h.logger.Debugf("Repository is %q and change request number is %d.", repository, prNumber)
	h.logger.Debugf("Webhook action is %q.", action)
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
