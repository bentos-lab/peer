package wiring

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	githubinbound "github.com/bentos-lab/peer/adapter/inbound/http/github"
	customrecipe "github.com/bentos-lab/peer/adapter/outbound/customrecipe"
	githubvcs "github.com/bentos-lab/peer/adapter/outbound/vcs/github"
	"github.com/bentos-lab/peer/config"
	"github.com/bentos-lab/peer/shared/jobqueue"
	"github.com/bentos-lab/peer/usecase"
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

	reviewBuilder := func(repoURL string) (usecase.ReviewUseCase, error) {
		return BuildReviewUseCase(cfgWithOverrides, CLILLMOptions{}, "")
	}
	overviewBuilder := func(repoURL string) (usecase.OverviewUseCase, error) {
		return BuildOverviewUseCase(cfgWithOverrides, CLILLMOptions{}, "")
	}
	autogenBuilder := func(repoURL string) (usecase.AutogenUseCase, error) {
		return BuildAutogenUseCase(cfgWithOverrides, CLILLMOptions{}, "")
	}
	replyBuilder := func(repoURL string) (usecase.ReplyCommentUseCase, error) {
		return BuildReplyCommentUseCase(cfgWithOverrides, CLILLMOptions{}, "")
	}
	configLoader, err := customrecipe.NewConfigLoader(deps.CodeEnvironmentFactory, logger)
	if err != nil {
		return nil, err
	}
	queue := jobqueue.NewManager(cfg.Server.MaxJobWorkers)
	return githubinbound.NewHandler(
		reviewBuilder,
		overviewBuilder,
		autogenBuilder,
		replyBuilder,
		ghClient,
		configLoader,
		deps.CodeEnvironmentFactory,
		deps.RecipeLoader,
		logger,
		cfg.Server.GitHub.WebhookSecret,
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
	), nil
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
