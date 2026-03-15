package wiring

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	githubinbound "bentos-backend/adapter/inbound/http/github"
	customrecipe "bentos-backend/adapter/outbound/customrecipe"
	githubvcs "bentos-backend/adapter/outbound/vcs/github"
	"bentos-backend/config"
	"bentos-backend/usecase"
)

// BuildGitHubHandler wires dependencies for GitHub webhook flow.
func BuildGitHubHandler(cfg config.Config) (*githubinbound.Handler, error) {
	cfgWithOverrides := cfg
	cfgWithOverrides.CodingAgent = resolveCodingAgentConfig(cfg)

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

	deps, err := BuildCommonDependencies(cfgWithOverrides, CLILLMOptions{}, "")
	if err != nil {
		return nil, err
	}
	logger := deps.Logger

	changeRequestBuilder := func(repoURL string) (usecase.ChangeRequestUseCase, error) {
		return BuildChangeRequestUseCase(cfgWithOverrides, CLILLMOptions{}, "")
	}
	replyBuilder := func(repoURL string) (usecase.ReplyCommentUseCase, error) {
		return BuildReplyCommentUseCase(cfgWithOverrides, CLILLMOptions{}, "")
	}
	configLoader, err := customrecipe.NewConfigLoader(deps.CodeEnvironmentFactory, logger)
	if err != nil {
		return nil, err
	}
	return githubinbound.NewHandler(
		changeRequestBuilder,
		replyBuilder,
		ghClient,
		configLoader,
		deps.CodeEnvironmentFactory,
		deps.RecipeLoader,
		logger,
		cfg.Server.GitHub.WebhookSecret,
		cfg.Server.GitHub.ReplyCommentTriggerName,
		resolveOverviewEnabled(cfg),
		resolveSuggestionsEnabled(cfg),
	), nil
}

func resolveOverviewEnabled(cfg config.Config) bool {
	if cfg.OverviewEnabled == nil {
		return true
	}
	return *cfg.OverviewEnabled
}

func resolveSuggestionsEnabled(cfg config.Config) bool {
	return cfg.SuggestedChanges.Enabled
}

func resolveCodingAgentConfig(cfg config.Config) config.CodingAgentConfig {
	agent := strings.TrimSpace(cfg.CodingAgent.Agent)
	provider := strings.TrimSpace(cfg.CodingAgent.Provider)
	model := strings.TrimSpace(cfg.CodingAgent.Model)
	return config.CodingAgentConfig{
		Agent:    agent,
		Provider: provider,
		Model:    model,
	}
}
