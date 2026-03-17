package wiring

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	gitlabinbound "bentos-backend/adapter/inbound/http/gitlab"
	autogencodingagent "bentos-backend/adapter/outbound/autogen/codingagent"
	customrecipe "bentos-backend/adapter/outbound/customrecipe"
	issuealignment "bentos-backend/adapter/outbound/issuealignment/codeagent"
	overviewcodingagent "bentos-backend/adapter/outbound/overview/codingagent"
	gitlabpublisher "bentos-backend/adapter/outbound/publisher/gitlab"
	replycommentcodingagent "bentos-backend/adapter/outbound/replycomment/codingagent"
	reviewercodingagent "bentos-backend/adapter/outbound/reviewer/codingagent"
	gitlabvcs "bentos-backend/adapter/outbound/vcs/gitlab"
	"bentos-backend/config"
	"bentos-backend/shared/jobqueue"
	"bentos-backend/usecase"
	"bentos-backend/usecase/rulepack"
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
		return usecase.NewReviewUseCase(rulepack.NewCoreRulePackProvider(), reviewer, publisher, logger)
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
