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

	"github.com/bentos-lab/peer/adapter/inbound/cli"
	"github.com/bentos-lab/peer/adapter/inbound/http/background"
	codeenv "github.com/bentos-lab/peer/adapter/outbound/codeenv"
	githubvcs "github.com/bentos-lab/peer/adapter/outbound/vcs/github"
	"github.com/bentos-lab/peer/domain"
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
	GetPullRequestInfo(ctx context.Context, repository string, pullRequestNumber int) (domain.ChangeRequestInfo, error)
	GetIssue(ctx context.Context, repository string, issueNumber int) (domain.Issue, error)
	GetPullRequestReview(ctx context.Context, repository string, pullRequestNumber int, reviewID int64) (domain.ReviewSummary, error)
	ListIssueComments(ctx context.Context, repository string, pullRequestNumber int) ([]domain.IssueComment, error)
	ListReviewComments(ctx context.Context, repository string, pullRequestNumber int) ([]domain.ReviewComment, error)
}

// RecipeConfigLoader reads enabled toggles from the repo-scoped recipe config.
type RecipeConfigLoader interface {
	Load(ctx context.Context, repoURL string, headRef string) (domain.CustomRecipe, error)
}

// Handler receives GitHub webhook events and triggers review.
type Handler struct {
	reviewBuilder       ReviewUseCaseBuilder
	overviewBuilder     OverviewUseCaseBuilder
	autogenBuilder      AutogenUseCaseBuilder
	replyCommentBuilder ReplyCommentUseCaseBuilder
	tokenProvider       CommentClient
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

// NewHandler creates a GitHub webhook handler.
func NewHandler(
	reviewBuilder ReviewUseCaseBuilder,
	overviewBuilder OverviewUseCaseBuilder,
	autogenBuilder AutogenUseCaseBuilder,
	replyCommentBuilder ReplyCommentUseCaseBuilder,
	tokenProvider CommentClient,
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
		tokenProvider:       tokenProvider,
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
	reviewEnabled := h.reviewEnabled
	if recipeConfig.ReviewEnabled != nil {
		reviewEnabled = *recipeConfig.ReviewEnabled
	}

	reviewEnabled = reviewEnabled && isActionAllowed(event.Action, recipeConfig.ReviewEvents, h.reviewEvents)

	overviewEnabled := h.overviewEnabled
	if recipeConfig.OverviewEnabled != nil {
		overviewEnabled = *recipeConfig.OverviewEnabled
	}

	overviewEnabled = overviewEnabled && isActionAllowed(event.Action, recipeConfig.OverviewEvents, h.overviewEvents)

	enableSuggestions := cli.ResolveBool(recipeConfig.ReviewSuggestions, nil, h.reviewSuggestions)
	issueAlignmentEnabled := cli.ResolveBool(recipeConfig.OverviewIssueAlignmentEnabled, nil, h.overviewIssueAlign)

	autogenEnabled := h.autogenEnabled
	if recipeConfig.AutogenEnabled != nil {
		autogenEnabled = *recipeConfig.AutogenEnabled
	}

	autogenEnabled = autogenEnabled && isActionAllowed(event.Action, recipeConfig.AutogenEvents, h.autogenEvents)
	autogenDocs := cli.ResolveBool(recipeConfig.AutogenDocs, nil, h.autogenDocs)
	autogenTests := cli.ResolveBool(recipeConfig.AutogenTests, nil, h.autogenTests)
	if !autogenDocs && !autogenTests {
		autogenEnabled = false
	}

	if !reviewEnabled {
		h.logWebhookSkipped("review", event.Repository.FullName, event.PullRequest.Number, event.Action)
	}
	if !overviewEnabled {
		h.logWebhookSkipped("overview", event.Repository.FullName, event.PullRequest.Number, event.Action)
	}
	if !autogenEnabled {
		h.logWebhookSkipped("autogen", event.Repository.FullName, event.PullRequest.Number, event.Action)
	}

	if !reviewEnabled && !overviewEnabled && !autogenEnabled {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	var issueCandidates []domain.IssueContext
	if overviewEnabled && issueAlignmentEnabled {
		issueCandidates = resolveWebhookIssueCandidates(
			githubvcs.WithInstallationID(r.Context(), installationID),
			h.tokenProvider,
			event.Repository.FullName,
			event.PullRequest.Body,
		)
	}

	input := buildWebhookInput(
		event.Repository.FullName,
		event.PullRequest.Number,
		repoURL,
		base,
		head,
		event.PullRequest.Title,
		event.PullRequest.Body,
		event.Action,
	)

	var overviewJobID string
	if overviewEnabled {
		jobID, err := h.enqueueOverviewJob(installationID, event.Action, input, issueCandidates, head)
		if err != nil {
			http.Error(w, "failed to enqueue overview", http.StatusInternalServerError)
			return
		}
		overviewJobID = jobID
		h.logWebhookAccepted("overview", input.Target.Repository, input.Target.ChangeRequestNumber, event.Action)
	}

	if reviewEnabled {
		var deps []string
		if overviewJobID != "" {
			deps = []string{overviewJobID}
		}
		_, err := h.enqueueReviewJob(installationID, event.Action, input, enableSuggestions, head, deps)
		if err != nil {
			http.Error(w, "failed to enqueue review", http.StatusInternalServerError)
			return
		}
		h.logWebhookAccepted("review", input.Target.Repository, input.Target.ChangeRequestNumber, event.Action)
	}

	if autogenEnabled {
		headBranch := strings.TrimSpace(event.PullRequest.Head.Ref)
		_, err := h.enqueueAutogenJob(installationID, event.Action, input, autogenDocs, autogenTests, headBranch, head)
		if err != nil {
			http.Error(w, "failed to enqueue autogen", http.StatusInternalServerError)
			return
		}
		h.logWebhookAccepted("autogen", input.Target.Repository, input.Target.ChangeRequestNumber, event.Action)
	}

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
	replyEnabled := h.replyEnabled
	if recipeConfig.ReplyCommentEnabled != nil {
		replyEnabled = *recipeConfig.ReplyCommentEnabled
	}
	if !replyEnabled {
		h.logWebhookSkipped("replycomment", event.Repository.FullName, event.Issue.Number, event.Action)
		w.WriteHeader(http.StatusAccepted)
		return
	}
	if !isActionAllowed("issue_comment", recipeConfig.ReplyCommentEvents, h.replyEvents) {
		h.logWebhookSkipped("replycomment", event.Repository.FullName, event.Issue.Number, event.Action)
		w.WriteHeader(http.StatusAccepted)
		return
	}
	if !isActionAllowed(event.Action, recipeConfig.ReplyCommentActions, h.replyActions) {
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
	replyEnabled := h.replyEnabled
	if recipeConfig.ReplyCommentEnabled != nil {
		replyEnabled = *recipeConfig.ReplyCommentEnabled
	}
	if !replyEnabled {
		h.logWebhookSkipped("replycomment", event.Repository.FullName, event.PullRequest.Number, event.Action)
		w.WriteHeader(http.StatusAccepted)
		return
	}
	if !isActionAllowed("pull_request_review_comment", recipeConfig.ReplyCommentEvents, h.replyEvents) {
		h.logWebhookSkipped("replycomment", event.Repository.FullName, event.PullRequest.Number, event.Action)
		w.WriteHeader(http.StatusAccepted)
		return
	}
	if !isActionAllowed(event.Action, recipeConfig.ReplyCommentActions, h.replyActions) {
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

func (h *Handler) enqueueReviewJob(installationID string, action string, input domain.ChangeRequestInput, suggestions bool, headRef string, deps []string) (string, error) {
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
						"GitHub webhook review panicked for %q#%d action=%q after %d ms: %v.",
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
			ctx = githubvcs.WithInstallationID(ctx, installationID)
			if h.reviewBuilder == nil {
				return errors.New("review usecase builder is not configured")
			}
			if h.codeEnvFactory == nil || h.recipeLoader == nil {
				return errors.New("code environment is not configured")
			}
			environment, cleanup, err := codeenv.NewEnvironment(ctx, h.codeEnvFactory, input.RepoURL)
			if err != nil {
				return err
			}
			defer func() {
				if cleanupErr := cleanup(ctx); cleanupErr != nil {
					h.logger.Warnf("Failed to cleanup code environment: %v", cleanupErr)
				}
			}()

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
					"GitHub webhook review failed for %q#%d action=%q after %d ms.",
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

func (h *Handler) enqueueOverviewJob(installationID string, action string, input domain.ChangeRequestInput, issueCandidates []domain.IssueContext, headRef string) (string, error) {
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
						"GitHub webhook overview panicked for %q#%d action=%q after %d ms: %v.",
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
			ctx = githubvcs.WithInstallationID(ctx, installationID)
			if h.overviewBuilder == nil {
				return errors.New("overview usecase builder is not configured")
			}
			if h.codeEnvFactory == nil || h.recipeLoader == nil {
				return errors.New("code environment is not configured")
			}

			environment, cleanup, err := codeenv.NewEnvironment(ctx, h.codeEnvFactory, input.RepoURL)
			if err != nil {
				return err
			}
			defer func() {
				if cleanupErr := cleanup(ctx); cleanupErr != nil {
					h.logger.Warnf("Failed to cleanup code environment: %v", cleanupErr)
				}
			}()

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
					"GitHub webhook overview failed for %q#%d action=%q after %d ms.",
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

func (h *Handler) enqueueAutogenJob(installationID string, action string, input domain.ChangeRequestInput, docs bool, tests bool, headBranch string, headRef string) (string, error) {
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
						"GitHub webhook autogen panicked for %q#%d action=%q after %d ms: %v.",
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
			ctx = githubvcs.WithInstallationID(ctx, installationID)
			if h.autogenBuilder == nil {
				return errors.New("autogen usecase builder is not configured")
			}
			if h.codeEnvFactory == nil || h.recipeLoader == nil {
				return errors.New("code environment is not configured")
			}

			environment, cleanup, err := codeenv.NewEnvironment(ctx, h.codeEnvFactory, input.RepoURL)
			if err != nil {
				return err
			}
			defer func() {
				if cleanupErr := cleanup(ctx); cleanupErr != nil {
					h.logger.Warnf("Failed to cleanup code environment: %v", cleanupErr)
				}
			}()

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
					"GitHub webhook autogen failed for %q#%d action=%q after %d ms.",
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
