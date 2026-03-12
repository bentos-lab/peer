package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"bentos-backend/config"
	sharedcli "bentos-backend/shared/cli"
	"bentos-backend/wiring"

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
	logLevel                                string
	overviewEnabled                         bool
	reviewSuggestedChangesEnabled           bool
	reviewSuggestedChangesMinSeverity       string
	reviewSuggestedChangesMaxCandidates     int
	reviewSuggestedChangesMaxGroupSize      int
	reviewSuggestedChangesMaxWorkers        int
	reviewSuggestedChangesGroupTimeoutMS    int
	reviewSuggestedChangesGenerateTimeoutMS int
	port                                    string
	githubWebhookSecret                     string
	githubAppID                             string
	githubAppPrivateKey                     string
	githubAPIBaseURL                        string
	replycommentTriggerName                 string
	codingAgentName                         string
	codingAgentProvider                     string
	codingAgentModel                        string
}

func newWebhookSubcommand(
	_ context.Context,
	deps autogitDeps,
	llmOpenAIBaseURL *string,
	llmOpenAIModel *string,
	llmOpenAIAPIKey *string,
	verbosity *int,
	codeAgent *string,
	codeAgentProvider *string,
	codeAgentModel *string,
) *cobra.Command {
	var vcsProvider string
	flags := webhookFlagValues{}

	sub := &cobra.Command{
		Use:   "webhook",
		Short: "Run webhook server",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !flagChanged(cmd, "vcs-provider") || strings.TrimSpace(vcsProvider) == "" {
				_ = cmd.Help()
				return fmt.Errorf("at least one vcs provider is required")
			}
			providers, err := parseVCSProviders(vcsProvider)
			if err != nil {
				return err
			}
			for _, provider := range providers {
				if provider != "github" {
					return fmt.Errorf("unsupported vcs provider: %s", provider)
				}
			}
			if len(providers) == 0 {
				_ = cmd.Help()
				return fmt.Errorf("at least one vcs provider is required")
			}

			cfg, err := deps.loadConfig()
			if err != nil {
				return webhookConfigLoadError{cause: err}
			}

			logLevelOverride, err := resolveWebhookLogLevel(cmd, flags.logLevel, *verbosity)
			if err != nil {
				return err
			}
			if logLevelOverride != "" {
				cfg.LogLevel = logLevelOverride
			}

			if err := applyWebhookOverrides(cmd, &cfg, flags, llmOpenAIBaseURL, llmOpenAIModel, llmOpenAIAPIKey, codeAgent, codeAgentProvider, codeAgentModel); err != nil {
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

			startupLogger.Infof("webhook startup: wiring GitHub handler")
			githubHandler, err := deps.buildGitHubHandler(cfg)
			if err != nil {
				startupLogger.Errorf("webhook startup failed: build GitHub handler: %v", err)
				return fmt.Errorf("build GitHub handler: %w", err)
			}
			startupLogger.Infof("webhook startup: wired GitHub handler")

			mux := http.NewServeMux()
			mux.Handle("/webhook/github", githubHandler)
			startupLogger.Infof("webhook startup: route registered path=%q", "/webhook/github")

			addr := ":" + cfg.Server.Port
			startupLogger.Infof("webhook startup: listening on %q", addr)
			if err := deps.listenAndServe(addr, mux); err != nil {
				startupLogger.Errorf("webhook runtime failed: listen and serve: %v", err)
				return fmt.Errorf("listen and serve: %w", err)
			}

			return nil
		},
	}

	sub.Flags().StringVar(&vcsProvider, "vcs-provider", "", "vcs provider names joined by + (currently only github is supported)")
	sub.Flags().StringVar(&flags.logLevel, "log-level", "", "log level override (empty to use config or verbosity)")
	sub.Flags().BoolVar(&flags.overviewEnabled, "overview-enabled", false, "enable or disable overview generation")
	sub.Flags().BoolVar(&flags.reviewSuggestedChangesEnabled, "review-suggested-changes-enabled", false, "enable suggested code changes in review findings")
	sub.Flags().StringVar(&flags.reviewSuggestedChangesMinSeverity, "review-suggested-changes-min-severity", "", "minimum severity for suggested changes")
	sub.Flags().IntVar(&flags.reviewSuggestedChangesMaxCandidates, "review-suggested-changes-max-candidates", 0, "max candidates for suggested changes")
	sub.Flags().IntVar(&flags.reviewSuggestedChangesMaxGroupSize, "review-suggested-changes-max-group-size", 0, "max group size for suggested changes")
	sub.Flags().IntVar(&flags.reviewSuggestedChangesMaxWorkers, "review-suggested-changes-max-workers", 0, "max workers for suggested changes")
	sub.Flags().IntVar(&flags.reviewSuggestedChangesGroupTimeoutMS, "review-suggested-changes-group-timeout-ms", 0, "group timeout (ms) for suggested changes")
	sub.Flags().IntVar(&flags.reviewSuggestedChangesGenerateTimeoutMS, "review-suggested-changes-generate-timeout-ms", 0, "generate timeout (ms) for suggested changes")
	sub.Flags().StringVar(&flags.port, "port", "", "server port")
	sub.Flags().StringVar(&flags.githubWebhookSecret, "github-webhook-secret", "", "GitHub webhook secret")
	sub.Flags().StringVar(&flags.githubAppID, "github-app-id", "", "GitHub app id")
	sub.Flags().StringVar(&flags.githubAppPrivateKey, "github-app-private-key", "", "GitHub app private key")
	sub.Flags().StringVar(&flags.githubAPIBaseURL, "github-api-base-url", "", "GitHub API base URL")
	sub.Flags().StringVar(&flags.replycommentTriggerName, "replycomment-trigger-name", "", "replycomment trigger name")
	sub.Flags().StringVar(&flags.codingAgentName, "coding-agent-name", "", "coding agent name override")
	sub.Flags().StringVar(&flags.codingAgentProvider, "coding-agent-provider", "", "coding agent provider override")
	sub.Flags().StringVar(&flags.codingAgentModel, "coding-agent-model", "", "coding agent model override")

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
	if flagChanged(cmd, "overview-enabled") {
		value := flags.overviewEnabled
		cfg.OverviewEnabled = &value
	}
	if flagChanged(cmd, "review-suggested-changes-enabled") {
		cfg.SuggestedChanges.Enabled = flags.reviewSuggestedChangesEnabled
	}
	if flagChanged(cmd, "review-suggested-changes-min-severity") {
		cfg.SuggestedChanges.MinSeverity = strings.TrimSpace(flags.reviewSuggestedChangesMinSeverity)
	}
	if flagChanged(cmd, "review-suggested-changes-max-candidates") {
		if flags.reviewSuggestedChangesMaxCandidates <= 0 {
			return fmt.Errorf("flag --review-suggested-changes-max-candidates requires a positive value")
		}
		cfg.SuggestedChanges.MaxCandidates = flags.reviewSuggestedChangesMaxCandidates
	}
	if flagChanged(cmd, "review-suggested-changes-max-group-size") {
		if flags.reviewSuggestedChangesMaxGroupSize <= 0 {
			return fmt.Errorf("flag --review-suggested-changes-max-group-size requires a positive value")
		}
		cfg.SuggestedChanges.MaxGroupSize = flags.reviewSuggestedChangesMaxGroupSize
	}
	if flagChanged(cmd, "review-suggested-changes-max-workers") {
		if flags.reviewSuggestedChangesMaxWorkers <= 0 {
			return fmt.Errorf("flag --review-suggested-changes-max-workers requires a positive value")
		}
		cfg.SuggestedChanges.MaxWorkers = flags.reviewSuggestedChangesMaxWorkers
	}
	if flagChanged(cmd, "review-suggested-changes-group-timeout-ms") {
		if flags.reviewSuggestedChangesGroupTimeoutMS <= 0 {
			return fmt.Errorf("flag --review-suggested-changes-group-timeout-ms requires a positive value")
		}
		cfg.SuggestedChanges.GroupTimeoutMS = flags.reviewSuggestedChangesGroupTimeoutMS
	}
	if flagChanged(cmd, "review-suggested-changes-generate-timeout-ms") {
		if flags.reviewSuggestedChangesGenerateTimeoutMS <= 0 {
			return fmt.Errorf("flag --review-suggested-changes-generate-timeout-ms requires a positive value")
		}
		cfg.SuggestedChanges.GenerateTimeoutMS = flags.reviewSuggestedChangesGenerateTimeoutMS
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
		cfg.Server.GitHub.ReplyCommentTriggerName = strings.TrimSpace(flags.replycommentTriggerName)
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
