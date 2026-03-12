package wiring

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	githubinbound "bentos-backend/adapter/inbound/http/github"
	githubvcs "bentos-backend/adapter/outbound/vcs/github"
	"bentos-backend/config"
	"bentos-backend/usecase"
)

// BuildGitHubHandler wires dependencies for GitHub webhook flow.
func BuildGitHubHandler(cfg config.Config) (*githubinbound.Handler, error) {
	cfgWithOverrides := cfg
	cfgWithOverrides.CodingAgent = resolveCodingAgentConfig(cfg)
	logger, err := BuildLogger(cfgWithOverrides, "")
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

	changeRequestBuilder := func(repoURL string) (usecase.ChangeRequestUseCase, error) {
		return BuildChangeRequestUseCase(cfgWithOverrides, CLILLMOptions{}, "", repoURL)
	}
	replyBuilder := func(repoURL string) (usecase.ReplyCommentUseCase, error) {
		return BuildReplyCommentUseCase(cfgWithOverrides, CLILLMOptions{}, "", repoURL)
	}
	return githubinbound.NewHandler(
		changeRequestBuilder,
		replyBuilder,
		ghClient,
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
