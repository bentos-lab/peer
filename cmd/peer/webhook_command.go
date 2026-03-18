package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/bentos-lab/peer/config"
	sharedcli "github.com/bentos-lab/peer/shared/cli"
	"github.com/bentos-lab/peer/wiring"

	"github.com/spf13/cobra"
)

var errWebhookConfigLoad = errors.New("webhook config load failed")

type webhookConfigLoadError struct {
	cause error
}

func (e webhookConfigLoadError) Error() string {
	return fmt.Sprintf("load config: %v", e.cause)
}

func (e webhookConfigLoadError) Unwrap() error {
	return e.cause
}

func (e webhookConfigLoadError) Is(target error) bool {
	return target == errWebhookConfigLoad
}

type webhookFlagValues struct {
	logLevel                      string
	overviewEnabled               bool
	reviewSuggestedChangesEnabled bool
	port                          string
	githubWebhookSecret           string
	githubAppID                   string
	githubAppPrivateKey           string
	githubAPIBaseURL              string
	replycommentTriggerName       string
	codingAgentName               string
	codingAgentProvider           string
	codingAgentModel              string
}

func newWebhookSubcommand(
	_ context.Context,
	deps peerDeps,
) *cobra.Command {
	var vcsProvider string
	flags := webhookFlagValues{}
	var llmOpenAIBaseURL string
	var llmOpenAIModel string
	var llmOpenAIAPIKey string
	var codeAgent string
	var codeAgentProvider string
	var codeAgentModel string
	var verbosity int

	sub := &cobra.Command{
		Use:   "webhook",
		Short: "Run webhook server",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !flagChanged(cmd, "vcs-provider") || strings.TrimSpace(vcsProvider) == "" {
				return fmt.Errorf("at least one vcs provider is required (supported: %s)", sharedcli.SupportedVCSProviderValuesText())
			}
			providers, err := parseVCSProviders(vcsProvider)
			if err != nil {
				return err
			}
			if len(providers) == 0 {
				return fmt.Errorf("at least one vcs provider is required (supported: %s)", sharedcli.SupportedVCSProviderValuesText())
			}
			providerSet := make(map[string]struct{}, len(providers))
			for _, provider := range providers {
				parsedProvider, _, err := sharedcli.ParseVCSProvider(provider)
				if err != nil {
					return err
				}
				switch parsedProvider {
				case "github", "gitlab":
					providerSet[parsedProvider] = struct{}{}
				default:
					return fmt.Errorf("unsupported vcs provider: %s (supported: %s)", parsedProvider, sharedcli.SupportedVCSProviderValuesText())
				}
			}

			cfg, err := deps.loadConfig()
			if err != nil {
				return webhookConfigLoadError{cause: err}
			}

			logLevelOverride, err := resolveWebhookLogLevel(cmd, flags.logLevel, verbosity)
			if err != nil {
				return err
			}
			if logLevelOverride != "" {
				cfg.LogLevel = logLevelOverride
			}

			if err := applyWebhookOverrides(cmd, &cfg, flags, &llmOpenAIBaseURL, &llmOpenAIModel, &llmOpenAIAPIKey, &codeAgent, &codeAgentProvider, &codeAgentModel); err != nil {
				return err
			}

			startupLogger, err := wiring.BuildLogger(cfg, "")
			if err != nil {
				return err
			}
			startupLogger.Infof("webhook startup: config loaded; port=%q log-level=%q", cfg.Server.Port, cfg.LogLevel)
			selection, err := wiring.ResolveLLMSelection(cfg, wiring.CLILLMOptions{})
			if err != nil {
				startupLogger.Errorf("webhook startup failed: resolve llm selection: %v", err)
				return fmt.Errorf("resolve llm selection: %w", err)
			}
			if selection.UseOpenAI {
				startupLogger.Infof(
					`webhook startup: llm=openai base_url=%q model=%q`,
					selection.OpenAI.BaseURL,
					selection.OpenAI.Model,
				)
			} else {
				startupLogger.Infof(
					`webhook startup: llm=codingagent agent=%q provider=%q model=%q`,
					cfg.CodingAgent.Agent,
					cfg.CodingAgent.Provider,
					cfg.CodingAgent.Model,
				)
			}

			mux := http.NewServeMux()
			if _, ok := providerSet["github"]; ok {
				startupLogger.Infof("webhook startup: wiring GitHub handler")
				githubHandler, err := deps.buildGitHubHandler(cfg)
				if err != nil {
					startupLogger.Errorf("webhook startup failed: build GitHub handler: %v", err)
					return fmt.Errorf("build GitHub handler: %w", err)
				}
				startupLogger.Infof("webhook startup: wired GitHub handler")
				mux.Handle("/webhook/github", githubHandler)
				startupLogger.Infof("webhook startup: route registered path=%q", "/webhook/github")
			}
			if _, ok := providerSet["gitlab"]; ok {
				startupLogger.Infof("webhook startup: wiring GitLab handler")
				gitlabHandler, syncer, err := deps.buildGitLabHandler(cfg)
				if err != nil {
					startupLogger.Errorf("webhook startup failed: build GitLab handler: %v", err)
					return fmt.Errorf("build GitLab handler: %w", err)
				}
				startupLogger.Infof("webhook startup: wired GitLab handler")
				const gitlabWebhookPath = "/webhook/gitlab"
				mux.Handle(gitlabWebhookPath, gitlabHandler)
				startupLogger.Infof("webhook startup: route registered path=%q", gitlabWebhookPath)
				if syncer != nil {
					go syncer.Start(context.Background())
				}
			}

			addr := ":" + cfg.Server.Port
			startupLogger.Infof("webhook startup: listening on %q", addr)
			if err := deps.listenAndServe(addr, mux); err != nil {
				startupLogger.Errorf("webhook runtime failed: listen and serve: %v", err)
				return fmt.Errorf("listen and serve: %w", err)
			}

			return nil
		},
	}

	sub.Flags().StringVar(&vcsProvider, "vcs-provider", "", sharedcli.VCSProviderListFlagHelp())
	sub.Flags().StringVar(&flags.logLevel, "log-level", "", "log level override (empty to use config or verbosity, env: LOG_LEVEL)")
	sub.Flags().BoolVar(&flags.overviewEnabled, "overview", false, "enable or disable overview generation (env: OVERVIEW)")
	sub.Flags().BoolVar(&flags.reviewSuggestedChangesEnabled, "review-suggested-changes", false, "enable suggested code changes in review findings (env: REVIEW_SUGGESTED_CHANGES)")
	sub.Flags().StringVar(&flags.port, "port", "", "server port (env: PORT)")
	sub.Flags().StringVar(&flags.githubWebhookSecret, "github-webhook-secret", "", "GitHub webhook secret (env: GITHUB_WEBHOOK_SECRET)")
	sub.Flags().StringVar(&flags.githubAppID, "github-app-id", "", "GitHub app id (env: GITHUB_APP_ID)")
	sub.Flags().StringVar(&flags.githubAppPrivateKey, "github-app-private-key", "", "GitHub app private key (env: GITHUB_APP_PRIVATE_KEY)")
	sub.Flags().StringVar(&flags.githubAPIBaseURL, "github-api-base-url", "", "GitHub API base URL (env: GITHUB_API_BASE_URL)")
	sub.Flags().StringVar(&flags.replycommentTriggerName, "replycomment-trigger-name", "", "replycomment trigger name (env: REPLYCOMMENT_TRIGGER_NAME)")
	sub.Flags().StringVar(&flags.codingAgentName, "coding-agent-name", "", "coding agent name override (env: CODING_AGENT_NAME)")
	sub.Flags().StringVar(&flags.codingAgentProvider, "coding-agent-provider", "", "coding agent provider override (env: CODING_AGENT_PROVIDER)")
	sub.Flags().StringVar(&flags.codingAgentModel, "coding-agent-model", "", "coding agent model override (env: CODING_AGENT_MODEL)")
	sub.Flags().StringVar(&llmOpenAIBaseURL, "llm-openai-base-url", "", "OpenAI compatible base URL override (empty to use coding-agent LLM, env: LLM_OPENAI_BASE_URL)")
	sub.Flags().StringVar(&llmOpenAIModel, "llm-openai-model", "", "OpenAI compatible model override (env: LLM_OPENAI_MODEL)")
	sub.Flags().StringVar(&llmOpenAIAPIKey, "llm-openai-api-key", "", "OpenAI compatible API key override (env: LLM_OPENAI_API_KEY)")
	sub.Flags().StringVar(&codeAgent, "code-agent", "", "coding agent override (empty to use config, env: CODING_AGENT_NAME)")
	sub.Flags().StringVar(&codeAgentProvider, "code-agent-provider", "", "coding agent provider override (empty to use config, env: CODING_AGENT_PROVIDER)")
	sub.Flags().StringVar(&codeAgentModel, "code-agent-model", "", "coding agent model override (empty to use config, env: CODING_AGENT_MODEL)")
	sub.Flags().CountVarP(&verbosity, "verbose", "v", "increase log verbosity (-v=debug, -vv=trace, default=info)")

	return sub
}

func parseVCSProviders(raw string) ([]string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, fmt.Errorf("at least one vcs provider is required")
	}
	parts := strings.Split(trimmed, "+")
	seen := make(map[string]struct{}, len(parts))
	var providers []string
	for _, part := range parts {
		name := strings.TrimSpace(part)
		if name == "" {
			return nil, fmt.Errorf("invalid vcs provider value")
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		providers = append(providers, name)
	}
	return providers, nil
}

func resolveWebhookLogLevel(cmd *cobra.Command, logLevel string, verbosity int) (string, error) {
	if flagChanged(cmd, "log-level") {
		value := strings.TrimSpace(logLevel)
		if value == "" {
			return "", fmt.Errorf("flag --log-level requires a non-empty value")
		}
		return value, nil
	}
	return sharedcli.LogLevelOverrideFromVerbosity(verbosity), nil
}

func applyWebhookOverrides(
	cmd *cobra.Command,
	cfg *config.Config,
	flags webhookFlagValues,
	llmOpenAIBaseURL *string,
	llmOpenAIModel *string,
	llmOpenAIAPIKey *string,
	codeAgent *string,
	codeAgentProvider *string,
	codeAgentModel *string,
) error {
	if flagChanged(cmd, "overview") {
		cfg.Overview.Enabled = flags.overviewEnabled
	}
	if flagChanged(cmd, "review-suggested-changes") {
		cfg.Review.SuggestedChangesEnabled = flags.reviewSuggestedChangesEnabled
	}
	if flagChanged(cmd, "port") {
		value := strings.TrimSpace(flags.port)
		if value == "" {
			return fmt.Errorf("flag --port requires a non-empty value")
		}
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed <= 0 || parsed > 65535 {
			return fmt.Errorf("flag --port requires a valid port (1-65535)")
		}
		cfg.Server.Port = value
	}
	if flagChanged(cmd, "github-webhook-secret") {
		cfg.Server.GitHub.WebhookSecret = strings.TrimSpace(flags.githubWebhookSecret)
	}
	if flagChanged(cmd, "github-app-id") {
		cfg.Server.GitHub.AppID = strings.TrimSpace(flags.githubAppID)
	}
	if flagChanged(cmd, "github-app-private-key") {
		cfg.Server.GitHub.AppPrivateKey = strings.TrimSpace(flags.githubAppPrivateKey)
	}
	if flagChanged(cmd, "github-api-base-url") {
		cfg.Server.GitHub.APIBaseURL = strings.TrimSpace(flags.githubAPIBaseURL)
	}
	if flagChanged(cmd, "replycomment-trigger-name") {
		cfg.ReplyComment.TriggerName = strings.TrimSpace(flags.replycommentTriggerName)
	}
	if flagChanged(cmd, "llm-openai-base-url") {
		cfg.OpenAI.BaseURL = strings.TrimSpace(*llmOpenAIBaseURL)
	}
	if flagChanged(cmd, "llm-openai-model") {
		cfg.OpenAI.Model = strings.TrimSpace(*llmOpenAIModel)
	}
	if flagChanged(cmd, "llm-openai-api-key") {
		cfg.OpenAI.APIKey = strings.TrimSpace(*llmOpenAIAPIKey)
	}
	if flagChanged(cmd, "coding-agent-name") {
		cfg.CodingAgent.Agent = strings.TrimSpace(flags.codingAgentName)
	}
	if flagChanged(cmd, "coding-agent-provider") {
		cfg.CodingAgent.Provider = strings.TrimSpace(flags.codingAgentProvider)
	}
	if flagChanged(cmd, "coding-agent-model") {
		cfg.CodingAgent.Model = strings.TrimSpace(flags.codingAgentModel)
	}
	if flagChanged(cmd, "code-agent") {
		cfg.CodingAgent.Agent = strings.TrimSpace(*codeAgent)
	}
	if flagChanged(cmd, "code-agent-provider") {
		cfg.CodingAgent.Provider = strings.TrimSpace(*codeAgentProvider)
	}
	if flagChanged(cmd, "code-agent-model") {
		cfg.CodingAgent.Model = strings.TrimSpace(*codeAgentModel)
	}
	return nil
}
