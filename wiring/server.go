package wiring

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	githubinbound "bentos-backend/adapter/inbound/http/github"
	codeenvhost "bentos-backend/adapter/outbound/codeenv/host"
	llmtracing "bentos-backend/adapter/outbound/llm/tracing"
	overviewcodingagent "bentos-backend/adapter/outbound/overview/codingagent"
	githubpublisher "bentos-backend/adapter/outbound/publisher/github"
	replycommentcodingagent "bentos-backend/adapter/outbound/replycomment/codingagent"
	replycommentsanitizer "bentos-backend/adapter/outbound/replycomment/sanitizer"
	reviewercodingagent "bentos-backend/adapter/outbound/reviewer/codingagent"
	githubvcs "bentos-backend/adapter/outbound/vcs/github"
	"bentos-backend/config"
	"bentos-backend/usecase"
	"bentos-backend/usecase/contracts"
	"bentos-backend/usecase/rulepack"
)

type codingAgentRuntimeConfig struct {
	Agent    string
	Provider string
	Model    string
}

// BuildGitHubHandler wires dependencies for GitHub webhook flow.
func BuildGitHubHandler(cfg config.Config) (*githubinbound.Handler, error) {
	logger, err := buildLogger(cfg, "")
	if err != nil {
		return nil, err
	}
	llmSelection, err := ResolveLLMSelection(cfg, CLILLMOptions{})
	if err != nil {
		return nil, err
	}
	var formatterClient contracts.LLMGenerator
	if llmSelection.UseOpenAI {
		formatterClient = buildOpenAIGenerator(llmSelection)
	} else {
		formatterClient, err = buildCodingAgentGenerator(cfg, logger)
		if err != nil {
			return nil, err
		}
	}
	tracedFormatter := llmtracing.NewGenerator(formatterClient, logger)
	codeEnvironmentFactory := codeenvhost.NewFactory(codeenvhost.FactoryConfig{
		Logger: logger,
	})
	codingAgentConfig := resolveServerCodingAgentConfig(cfg)

	codingReviewer, err := reviewercodingagent.NewReviewer(tracedFormatter, reviewercodingagent.Config{
		Agent:    codingAgentConfig.Agent,
		Provider: codingAgentConfig.Provider,
		Model:    codingAgentConfig.Model,
	}, logger)
	if err != nil {
		return nil, err
	}
	codingOverview, err := overviewcodingagent.NewOverviewGenerator(tracedFormatter, overviewcodingagent.Config{
		Agent:    codingAgentConfig.Agent,
		Provider: codingAgentConfig.Provider,
		Model:    codingAgentConfig.Model,
	}, logger)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(cfg.Server.GitHub.WebhookSecret) == "" {
		return nil, fmt.Errorf("github webhook secret is required")
	}
	httpClient := &http.Client{Timeout: 60 * time.Second}
	ghClient, err := githubvcs.NewAppClient(httpClient, githubvcs.AppClientConfig{
		APIBaseURL: cfg.Server.GitHub.APIBaseURL,
		AppID:      cfg.Server.GitHub.AppID,
		PrivateKey: cfg.Server.GitHub.AppPrivateKey,
	})
	if err != nil {
		return nil, err
	}
	reviewUseCase, err := usecase.NewReviewUseCase(
		rulepack.NewCoreRulePackProvider(),
		codingReviewer,
		githubpublisher.NewPublisher(ghClient, logger),
		codeEnvironmentFactory,
		logger,
	)
	if err != nil {
		return nil, err
	}
	overviewUseCase, err := usecase.NewOverviewUseCase(
		codingOverview,
		githubpublisher.NewOverviewPublisher(ghClient, logger),
		codeEnvironmentFactory,
		logger,
	)
	if err != nil {
		return nil, err
	}
	changeRequestUseCase, err := usecase.NewChangeRequestUseCase(
		reviewUseCase,
		overviewUseCase,
		logger,
	)
	if err != nil {
		return nil, err
	}
	replyAnswerer, err := replycommentcodingagent.NewAnswerer(replycommentcodingagent.Config{
		Agent:    codingAgentConfig.Agent,
		Provider: codingAgentConfig.Provider,
		Model:    codingAgentConfig.Model,
	}, logger)
	if err != nil {
		return nil, err
	}
	replySanitizer, err := replycommentsanitizer.NewSanitizer(tracedFormatter)
	if err != nil {
		return nil, err
	}
	replyPublisher := githubpublisher.NewReplyCommentPublisher(ghClient, logger)
	replyUseCase, err := usecase.NewReplyCommentUseCase(
		replySanitizer,
		replyAnswerer,
		replyPublisher,
		codeEnvironmentFactory,
		logger,
	)
	if err != nil {
		return nil, err
	}
	return githubinbound.NewHandler(
		changeRequestUseCase,
		replyUseCase,
		ghClient,
		logger,
		cfg.Server.GitHub.WebhookSecret,
		cfg.Server.GitHub.ReplyCommentTriggerName,
		resolveServerOverviewEnabled(cfg),
		resolveServerSuggestionsEnabled(cfg),
	), nil
}

func resolveServerOverviewEnabled(cfg config.Config) bool {
	if cfg.OverviewEnabled == nil {
		return true
	}
	return *cfg.OverviewEnabled
}

func resolveServerSuggestionsEnabled(cfg config.Config) bool {
	return cfg.SuggestedChanges.Enabled
}

func resolveServerCodingAgentConfig(cfg config.Config) codingAgentRuntimeConfig {
	agent := strings.TrimSpace(cfg.CodingAgent.Agent)
	provider := strings.TrimSpace(cfg.CodingAgent.Provider)
	model := strings.TrimSpace(cfg.CodingAgent.Model)
	return codingAgentRuntimeConfig{
		Agent:    agent,
		Provider: provider,
		Model:    model,
	}
}
