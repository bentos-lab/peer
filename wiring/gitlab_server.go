package wiring

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	gitlabinbound "github.com/bentos-lab/peer/adapter/inbound/http/gitlab"
	autogencodingagent "github.com/bentos-lab/peer/adapter/outbound/autogen/codingagent"
	customrecipe "github.com/bentos-lab/peer/adapter/outbound/customrecipe"
	issuealignment "github.com/bentos-lab/peer/adapter/outbound/issuealignment/codeagent"
	overviewcodingagent "github.com/bentos-lab/peer/adapter/outbound/overview/codingagent"
	gitlabpublisher "github.com/bentos-lab/peer/adapter/outbound/publisher/gitlab"
	replycommentcodingagent "github.com/bentos-lab/peer/adapter/outbound/replycomment/codingagent"
	reviewercodingagent "github.com/bentos-lab/peer/adapter/outbound/reviewer/codingagent"
	gitlabvcs "github.com/bentos-lab/peer/adapter/outbound/vcs/gitlab"
	"github.com/bentos-lab/peer/config"
	"github.com/bentos-lab/peer/shared/jobqueue"
	"github.com/bentos-lab/peer/usecase"
)

// BuildGitLabHandler wires dependencies for GitLab webhook flow.
func BuildGitLabHandler(cfg config.Config) (*gitlabinbound.Handler, *gitlabinbound.HookSyncer, error) {
	cfgWithOverrides := cfg
	cfgWithOverrides.CodingAgent = resolveCodingAgentConfig(cfg)

	if strings.TrimSpace(cfg.Server.GitLab.WebhookSecret) == "" {
		return nil, nil, fmt.Errorf("gitlab webhook secret is required")
	}
	if strings.TrimSpace(cfg.Server.GitLab.Token) == "" {
		return nil, nil, fmt.Errorf("gitlab token is required")
	}

	httpClient := &http.Client{Timeout: 60 * time.Second}
	glClient, err := gitlabvcs.NewAPIClient(httpClient, gitlabvcs.APIClientConfig{
		BaseURL: cfg.Server.GitLab.APIBaseURL,
		Token:   cfg.Server.GitLab.Token,
	})
	if err != nil {
		return nil, nil, err
	}

	deps, err := BuildCommonDependencies(cfgWithOverrides, CLILLMOptions{}, "")
	if err != nil {
		return nil, nil, err
	}
	logger := deps.Logger

	reviewBuilder := func(repoURL string) (usecase.ReviewUseCase, error) {
		reviewer, err := reviewercodingagent.NewReviewer(deps.TracedGenerator, reviewercodingagent.Config{
			Agent:    deps.CodingAgentConfig.Agent,
			Provider: deps.CodingAgentConfig.Provider,
			Model:    deps.CodingAgentConfig.Model,
		}, logger)
		if err != nil {
			return nil, err
		}
		publisher := gitlabpublisher.NewPublisher(glClient, logger)
		return usecase.NewReviewUseCase(reviewer, publisher, logger)
	}

	overviewBuilder := func(repoURL string) (usecase.OverviewUseCase, error) {
		overviewGenerator, err := overviewcodingagent.NewOverviewGenerator(deps.TracedGenerator, overviewcodingagent.Config{
			Agent:    deps.CodingAgentConfig.Agent,
			Provider: deps.CodingAgentConfig.Provider,
			Model:    deps.CodingAgentConfig.Model,
		}, logger)
		if err != nil {
			return nil, err
		}
		issueAlignmentGenerator, err := issuealignment.NewIssueAlignmentGenerator(deps.TracedGenerator, issuealignment.Config{
			Agent:    deps.CodingAgentConfig.Agent,
			Provider: deps.CodingAgentConfig.Provider,
			Model:    deps.CodingAgentConfig.Model,
		}, logger)
		if err != nil {
			return nil, err
		}
		publisher := gitlabpublisher.NewOverviewPublisher(glClient, logger)
		return usecase.NewOverviewUseCase(overviewGenerator, issueAlignmentGenerator, publisher, logger)
	}

	autogenBuilder := func(repoURL string) (usecase.AutogenUseCase, error) {
		generator, err := autogencodingagent.NewGenerator(autogencodingagent.Config{
			Agent:    deps.CodingAgentConfig.Agent,
			Provider: deps.CodingAgentConfig.Provider,
			Model:    deps.CodingAgentConfig.Model,
		}, logger)
		if err != nil {
			return nil, err
		}
		publisher := gitlabpublisher.NewAutogenPublisher(glClient, logger)
		return usecase.NewAutogenUseCase(generator, publisher, logger)
	}

	replyBuilder := func(repoURL string) (usecase.ReplyCommentUseCase, error) {
		answerer, err := replycommentcodingagent.NewAnswerer(replycommentcodingagent.Config{
			Agent:    deps.CodingAgentConfig.Agent,
			Provider: deps.CodingAgentConfig.Provider,
			Model:    deps.CodingAgentConfig.Model,
		}, logger)
		if err != nil {
			return nil, err
		}
		publisher := gitlabpublisher.NewReplyCommentPublisher(glClient, logger)
		return usecase.NewReplyCommentUseCase(deps.ReadOnlySanitizer, answerer, publisher, logger)
	}

	configLoader, err := customrecipe.NewConfigLoader(deps.CodeEnvironmentFactory, logger)
	if err != nil {
		return nil, nil, err
	}
	queue := jobqueue.NewManager(cfg.Server.MaxJobWorkers)

	handler := gitlabinbound.NewHandler(
		reviewBuilder,
		overviewBuilder,
		autogenBuilder,
		replyBuilder,
		glClient,
		configLoader,
		deps.CodeEnvironmentFactory,
		deps.RecipeLoader,
		logger,
		cfg.Server.GitLab.WebhookSecret,
		cfg.ReplyComment.TriggerName,
		cfg.Review.Enabled,
		cfg.Review.Events,
		cfg.Review.SuggestedChangesEnabled,
		cfg.Overview.Enabled,
		cfg.Overview.Events,
		cfg.Overview.IssueAlignmentEnabled,
		cfg.Autogen.Enabled,
		cfg.Autogen.Events,
		cfg.Autogen.DocsEnabled,
		cfg.Autogen.TestsEnabled,
		cfg.ReplyComment.Enabled,
		cfg.ReplyComment.Events,
		cfg.ReplyComment.Actions,
		queue,
	)

	syncer := gitlabinbound.NewHookSyncer(
		glClient,
		logger,
		cfg.Server.GitLab.WebhookURL,
		cfg.Server.GitLab.WebhookSecret,
		time.Duration(cfg.Server.GitLab.SyncInterval)*time.Minute,
		cfg.Server.GitLab.SyncStatePath,
	)

	return handler, syncer, nil
}
