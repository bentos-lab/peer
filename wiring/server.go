package wiring

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	githubinbound "bentos-backend/adapter/inbound/http/github"
	codeenvhost "bentos-backend/adapter/outbound/codeenv/host"
	openai "bentos-backend/adapter/outbound/llm/openai"
	llmtracing "bentos-backend/adapter/outbound/llm/tracing"
	overviewcodingagent "bentos-backend/adapter/outbound/overview/codingagent"
	githubpublisher "bentos-backend/adapter/outbound/publisher/github"
	reviewercodingagent "bentos-backend/adapter/outbound/reviewer/codingagent"
	githubvcs "bentos-backend/adapter/outbound/vcs/github"
	"bentos-backend/config"
	"bentos-backend/usecase"
	"bentos-backend/usecase/rulepack"
)

const serverLLMTimeout = 600 * time.Second

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
	openAIConfig, err := buildOpenAIClientConfig(cfg)
	if err != nil {
		return nil, err
	}
	formatterClient := openai.NewClient(&http.Client{Timeout: serverLLMTimeout}, openAIConfig)
	tracedFormatter := llmtracing.NewGenerator(formatterClient, logger)
	codeEnvironmentFactory := codeenvhost.NewFactory(logger)
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
	return githubinbound.NewHandler(
		changeRequestUseCase,
		ghClient,
		logger,
		cfg.Server.GitHub.WebhookSecret,
		resolveServerOverviewEnabled(cfg),
		resolveServerSuggestionsEnabled(cfg),
	), nil
}

func buildOpenAIClientConfig(cfg config.Config) (openai.ClientConfig, error) {
	effectiveConfig, err := ResolveEffectiveOpenAIConfig(cfg, CLILLMOptions{})
	if err != nil {
		return openai.ClientConfig{}, err
	}

	return openai.ClientConfig{
		BaseURL: effectiveConfig.BaseURL,
		APIKey:  cfg.OpenAI.APIKey,
		Model:   effectiveConfig.Model,
	}, nil
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
	if agent == "" {
		agent = "opencode"
	}
	provider := strings.TrimSpace(cfg.CodingAgent.Provider)
	if provider == "" {
		provider = "openai"
	}
	model := strings.TrimSpace(cfg.CodingAgent.Model)
	if model == "" {
		model = strings.TrimSpace(cfg.OpenAI.Model)
	}
	return codingAgentRuntimeConfig{
		Agent:    agent,
		Provider: provider,
		Model:    model,
	}
}
