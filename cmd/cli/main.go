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
		func(cfg config.Config, opts wiring.CLILLMOptions, logLevelOverride string) (*cliinbound.Command, error) {
			return wiring.BuildCLICommand(cfg, opts, logLevelOverride)
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
	buildCommand func(config.Config, wiring.CLILLMOptions, string) (*cliinbound.Command, error),
) error {
	root := newRootCommand(ctx, loadConfig, buildCommand)
	root.SetArgs(args)
	return root.ExecuteContext(ctx)
}

func newRootCommand(
	ctx context.Context,
	loadConfig func() (config.Config, error),
	buildCommand func(config.Config, wiring.CLILLMOptions, string) (*cliinbound.Command, error),
) *cobra.Command {
	var openAIBaseURL string
	var openAIModel string
	var openAIAPIKey string
	var provider string
	var repo string
	var changeRequest string
	var base string
	var head string
	var comment bool
	var overview bool
	var suggest bool
	var logLevel string

	var cmd *cobra.Command
	cmd = &cobra.Command{
		Use:   "autogit",
		Short: "Run repository review via GitHub context",
		RunE: func(_ *cobra.Command, _ []string) error {
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
			effectiveSuggest := false
			if cmd.Flags().Changed("suggest") {
				effectiveSuggest = suggest
			} else {
				effectiveSuggest = cfg.SuggestedChanges.Enabled
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

			cliCommand, err := buildCommand(cfg, opts, logLevel)
			if err != nil {
				return err
			}

			return cliCommand.Run(ctx, cliinbound.RunParams{
				Provider:      provider,
				Repo:          repo,
				ChangeRequest: changeRequest,
				Base:          base,
				Head:          head,
				Comment:       comment,
				Overview:      effectiveOverview,
				Suggest:       effectiveSuggest,
			})
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&openAIBaseURL, "openai-base-url", "gemini", "OpenAI compatible base URL override")
	flags.StringVar(&openAIModel, "openai-model", "", "OpenAI compatible model override")
	flags.StringVar(&openAIAPIKey, "openai-api-key", "", "OpenAI compatible API key override")
	flags.StringVar(&provider, "provider", "github", "provider name (only github is supported)")
	flags.StringVar(&repo, "repo", "", "repository (GitHub URL or owner/repo)")
	flags.StringVar(&changeRequest, "change-request", "", "GitHub pull request number")
	flags.StringVar(&base, "base", "", "base ref")
	flags.StringVar(&head, "head", "", "head ref or @staged/@all")
	flags.BoolVar(&comment, "comment", false, "post review result as pull request comments")
	flags.BoolVar(&overview, "overview", false, "generate and publish/print high-level overview output")
	flags.BoolVar(&suggest, "suggest", false, "enable suggested code changes in review findings")
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
