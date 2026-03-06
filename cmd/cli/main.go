package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	cliinbound "bentos-backend/adapter/inbound/cli"
	"bentos-backend/config"
	"bentos-backend/domain"
	stdlogger "bentos-backend/shared/logger/stdlogger"
	"bentos-backend/wiring"

	"github.com/spf13/cobra"
)

var errCLIConfigLoad = errors.New("cli config load failed")

type cliConfigLoadError struct {
	cause error
}

func (e cliConfigLoadError) Error() string {
	return fmt.Sprintf("load config: %v", e.cause)
}

func (e cliConfigLoadError) Unwrap() error {
	return e.cause
}

func (e cliConfigLoadError) Is(target error) bool {
	return target == errCLIConfigLoad
}

// main bootstraps the CLI review command.
func main() {
	if err := runCLI(
		context.Background(),
		os.Args[1:],
		config.Load,
		func(cfg config.Config, opts wiring.CLILLMOptions, provider domain.ChangeRequestInputProvider, publishType domain.ChangeRequestPublishType, logLevelOverride string) (*cliinbound.Command, error) {
			return wiring.BuildCLICommand(cfg, opts, provider, publishType, logLevelOverride)
		},
	); err != nil {
		if errors.Is(err, errCLIConfigLoad) {
			log.Printf("cli startup failed: %v", err)
		}
		os.Exit(1)
	}
}

func runCLI(
	ctx context.Context,
	args []string,
	loadConfig func() (config.Config, error),
	buildCommand func(config.Config, wiring.CLILLMOptions, domain.ChangeRequestInputProvider, domain.ChangeRequestPublishType, string) (*cliinbound.Command, error),
) error {
	root := newRootCommand(ctx, loadConfig, buildCommand)
	root.SetArgs(args)
	return root.ExecuteContext(ctx)
}

func newRootCommand(
	ctx context.Context,
	loadConfig func() (config.Config, error),
	buildCommand func(config.Config, wiring.CLILLMOptions, domain.ChangeRequestInputProvider, domain.ChangeRequestPublishType, string) (*cliinbound.Command, error),
) *cobra.Command {
	var openAIBaseURL string
	var openAIModel string
	var openAIAPIKey string
	var changedFiles string
	var includeUnstaged bool
	var includeUntracked bool
	var githubPRNumber int
	var commentOnPR bool
	var overview bool
	var logLevel string

	var cmd *cobra.Command
	cmd = &cobra.Command{
		Use:   "review",
		Short: "Run local repository review",
		RunE: func(_ *cobra.Command, _ []string) error {
			if err := validateReviewModeFlags(cmd, githubPRNumber, commentOnPR); err != nil {
				return err
			}

			opts := wiring.CLILLMOptions{}
			if cmd.Flags().Changed("openai-base-url") {
				value, err := validateOpenAIStringFlagValue("openai-base-url", openAIBaseURL)
				if err != nil {
					return err
				}
				opts.OpenAIBaseURL = value
			}
			if cmd.Flags().Changed("openai-model") {
				value, err := validateOpenAIStringFlagValue("openai-model", openAIModel)
				if err != nil {
					return err
				}
				opts.OpenAIModel = value
			}
			if cmd.Flags().Changed("openai-api-key") {
				value, err := validateOpenAIStringFlagValue("openai-api-key", openAIAPIKey)
				if err != nil {
					return err
				}
				opts.OpenAIAPIKey = value
			}
			if cmd.Flags().Changed("log-level") {
				value, err := validateLogLevelFlagValue(logLevel)
				if err != nil {
					return err
				}
				logLevel = value
			}

			cfg, err := loadConfig()
			if err != nil {
				return cliConfigLoadError{cause: err}
			}
			effectiveOverview := false
			if cmd.Flags().Changed("overview") {
				effectiveOverview = overview
			} else if cfg.OverviewEnabled != nil {
				effectiveOverview = *cfg.OverviewEnabled
			}
			startupLogger, err := wiring.BuildLogger(cfg, logLevel)
			if err != nil {
				return err
			}
			effectiveOpenAIConfig, err := wiring.ResolveEffectiveOpenAIConfig(cfg, opts)
			if err != nil {
				return err
			}
			startupLogger.Infof(
				`cli startup: llm_config base_url=%q model=%q`,
				effectiveOpenAIConfig.BaseURL,
				effectiveOpenAIConfig.Model,
			)

			provider, publishType := resolveCLISelection(githubPRNumber, commentOnPR)

			cliCommand, err := buildCommand(cfg, opts, provider, publishType, logLevel)
			if err != nil {
				return err
			}

			return cliCommand.Run(ctx, cliinbound.RunParams{
				ChangedFiles:     changedFiles,
				IncludeUnstaged:  includeUnstaged,
				IncludeUntracked: includeUntracked,
				PRNumber:         githubPRNumber,
				Overview:         effectiveOverview,
			})
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&openAIBaseURL, "openai-base-url", "gemini", "OpenAI compatible base URL override")
	flags.StringVar(&openAIModel, "openai-model", "", "OpenAI compatible model override")
	flags.StringVar(&openAIAPIKey, "openai-api-key", "", "OpenAI compatible API key override")
	flags.StringVarP(&changedFiles, "changed-files", "c", "", "comma-separated list of changed file paths")
	flags.BoolVarP(&includeUnstaged, "all", "a", false, "include unstaged changes")
	flags.BoolVarP(&includeUntracked, "untracked", "u", false, "include untracked files")
	flags.IntVar(&githubPRNumber, "gh-pr", 0, "GitHub pull request number to review")
	flags.BoolVar(&commentOnPR, "comment-on-pr", false, "post review result as comments on the GitHub pull request")
	flags.BoolVar(&overview, "overview", false, "generate and publish/print high-level overview output")
	flags.StringVar(&logLevel, "log-level", "", "log level override: trace|debug|info|warning|error|silence")

	return cmd
}

func validateLogLevelFlagValue(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Errorf("flag --log-level requires a non-empty value")
	}
	if _, ok := stdlogger.ParseLevel(trimmed); !ok {
		return "", fmt.Errorf("invalid --log-level %q: allowed values are trace, debug, info, warning, error, silence", trimmed)
	}
	return strings.ToLower(trimmed), nil
}

func validateOpenAIStringFlagValue(flagName string, value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || strings.HasPrefix(trimmed, "-") {
		return "", fmt.Errorf("flag --%s requires a non-empty value", flagName)
	}
	return trimmed, nil
}

func validateReviewModeFlags(cmd *cobra.Command, githubPRNumber int, commentOnPR bool) error {
	if githubPRNumber < 0 {
		return fmt.Errorf("flag --gh-pr requires a positive pull request number")
	}
	if githubPRNumber == 0 {
		if commentOnPR {
			return fmt.Errorf("flag --comment-on-pr requires --gh-pr")
		}
		return nil
	}

	if cmd.Flags().Changed("all") || cmd.Flags().Changed("untracked") || cmd.Flags().Changed("changed-files") {
		return fmt.Errorf("flags --all, --untracked, and --changed-files are not supported with --gh-pr")
	}

	return nil
}

func resolveCLISelection(githubPRNumber int, commentOnPR bool) (domain.ChangeRequestInputProvider, domain.ChangeRequestPublishType) {
	provider := domain.ChangeRequestInputProviderLocal
	publishType := domain.ChangeRequestPublishTypePrint

	if githubPRNumber > 0 {
		provider = domain.ChangeRequestInputProviderGitHub
	}
	if commentOnPR {
		publishType = domain.ChangeRequestPublishTypeComment
	}

	return provider, publishType
}
