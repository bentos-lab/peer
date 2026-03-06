package wiring

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	githubinbound "bentos-backend/adapter/inbound/http/github"
	gitlabinbound "bentos-backend/adapter/inbound/http/gitlab"
	githubinput "bentos-backend/adapter/outbound/input/github"
	gitlabinput "bentos-backend/adapter/outbound/input/gitlab"
	openai "bentos-backend/adapter/outbound/llm/openai"
	overviewllm "bentos-backend/adapter/outbound/overview/llm"
	githubpublisher "bentos-backend/adapter/outbound/publisher/github"
	gitlabpublisher "bentos-backend/adapter/outbound/publisher/gitlab"
	nooppublisher "bentos-backend/adapter/outbound/publisher/noop"
	reviewerllm "bentos-backend/adapter/outbound/reviewer/llm"
	githubvcs "bentos-backend/adapter/outbound/vcs/github"
	gitlabvcs "bentos-backend/adapter/outbound/vcs/gitlab"
	"bentos-backend/config"
	"bentos-backend/usecase"
	"bentos-backend/usecase/rulepack"
)

const serverLLMTimeout = 600 * time.Second

// BuildGitHubHandler wires dependencies for GitHub webhook flow.
func BuildGitHubHandler(cfg config.Config) (*githubinbound.Handler, error) {
	logger, err := buildLogger(cfg, "")
	if err != nil {
		return nil, err
	}
	llmReviewer, err := buildServerLLMReviewer(cfg, logger)
	if err != nil {
		return nil, err
	}
	llmOverview, err := buildServerLLMOverview(cfg, logger)
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
		llmReviewer,
		githubpublisher.NewPublisher(ghClient, logger),
		logger,
	)
	if err != nil {
		return nil, err
	}
	overviewUseCase, err := usecase.NewOverviewUseCase(
		llmOverview,
		githubpublisher.NewOverviewPublisher(ghClient, logger),
		logger,
	)
	if err != nil {
		return nil, err
	}
	changeRequestUseCase, err := usecase.NewChangeRequestUseCase(
		githubinput.NewProvider(ghClient),
		reviewUseCase,
		overviewUseCase,
		logger,
	)
	if err != nil {
		return nil, err
	}
	return githubinbound.NewHandler(changeRequestUseCase, logger, cfg.Server.GitHub.WebhookSecret, resolveServerOverviewEnabled(cfg)), nil
}

// BuildGitLabHandler wires dependencies for GitLab webhook flow.
func BuildGitLabHandler(cfg config.Config) (*gitlabinbound.Handler, error) {
	logger, err := buildLogger(cfg, "")
	if err != nil {
		return nil, err
	}
	llmReviewer, err := buildServerLLMReviewer(cfg, logger)
	if err != nil {
		return nil, err
	}
	llmOverview, err := buildServerLLMOverview(cfg, logger)
	if err != nil {
		return nil, err
	}
	glClient := gitlabvcs.NewClient()
	reviewUseCase, err := usecase.NewReviewUseCase(
		rulepack.NewCoreRulePackProvider(),
		llmReviewer,
		gitlabpublisher.NewPublisher(glClient, logger),
		logger,
	)
	if err != nil {
		return nil, err
	}
	overviewUseCase, err := usecase.NewOverviewUseCase(
		llmOverview,
		nooppublisher.NewOverviewPublisher(),
		logger,
	)
	if err != nil {
		return nil, err
	}
	changeRequestUseCase, err := usecase.NewChangeRequestUseCase(
		gitlabinput.NewProvider(glClient),
		reviewUseCase,
		overviewUseCase,
		logger,
	)
	if err != nil {
		return nil, err
	}
	return gitlabinbound.NewHandler(changeRequestUseCase, logger), nil
}

func buildServerLLMReviewer(cfg config.Config, logger usecase.Logger) (*reviewerllm.Reviewer, error) {
	httpClient := &http.Client{Timeout: serverLLMTimeout}
	openAIConfig, err := buildOpenAIClientConfig(cfg)
	if err != nil {
		return nil, err
	}
	llmClient := openai.NewClient(httpClient, openAIConfig)
	return reviewerllm.NewReviewer(llmClient, logger)
}

func buildServerLLMOverview(cfg config.Config, logger usecase.Logger) (*overviewllm.OverviewGenerator, error) {
	httpClient := &http.Client{Timeout: serverLLMTimeout}
	openAIConfig, err := buildOpenAIClientConfig(cfg)
	if err != nil {
		return nil, err
	}
	llmClient := openai.NewClient(httpClient, openAIConfig)
	return overviewllm.NewOverviewGenerator(llmClient, logger)
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
