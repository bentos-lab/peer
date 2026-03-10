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
	sharedcli "bentos-backend/shared/cli"
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
			return wiring.BuildCLIReviewCommand(cfg, opts, logLevelOverride)
		},
		func(cfg config.Config, opts wiring.CLILLMOptions, logLevelOverride string) (*cliinbound.ReplyCommentCommand, error) {
			return wiring.BuildCLIReplyCommentCommand(cfg, opts, logLevelOverride)
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
	buildReviewCommand func(config.Config, wiring.CLILLMOptions, string) (*cliinbound.Command, error),
	buildReplyCommentCommand func(config.Config, wiring.CLILLMOptions, string) (*cliinbound.ReplyCommentCommand, error),
) error {
	root := newRootCommand(ctx, loadConfig, buildReviewCommand, buildReplyCommentCommand)
	root.SetArgs(args)
	return root.ExecuteContext(ctx)
}

func newRootCommand(
	ctx context.Context,
	loadConfig func() (config.Config, error),
	buildReviewCommand func(config.Config, wiring.CLILLMOptions, string) (*cliinbound.Command, error),
	buildReplyCommentCommand func(config.Config, wiring.CLILLMOptions, string) (*cliinbound.ReplyCommentCommand, error),
) *cobra.Command {
	var openAIBaseURL string
	var openAIModel string
	var openAIAPIKey string
	var verbosity int

	var cmd *cobra.Command
	cmd = &cobra.Command{
		Use:   "autogit",
		Short: "Run repository review via GitHub context",
		RunE: func(_ *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	persistentFlags := cmd.PersistentFlags()
	persistentFlags.StringVar(&openAIBaseURL, "openai-base-url", "gemini", "OpenAI compatible base URL override")
	persistentFlags.StringVar(&openAIModel, "openai-model", "", "OpenAI compatible model override")
	persistentFlags.StringVar(&openAIAPIKey, "openai-api-key", "", "OpenAI compatible API key override")
	persistentFlags.CountVarP(&verbosity, "verbose", "v", "increase log verbosity (-v=info, -vv=debug, -vvv=trace)")

	cmd.AddCommand(newReviewSubcommand(ctx, loadConfig, buildReviewCommand, &openAIBaseURL, &openAIModel, &openAIAPIKey, &verbosity))
	cmd.AddCommand(newReplyCommentSubcommand(ctx, loadConfig, buildReplyCommentCommand, &openAIBaseURL, &openAIModel, &openAIAPIKey, &verbosity))

	return cmd
}

func newReviewSubcommand(
	ctx context.Context,
	loadConfig func() (config.Config, error),
	buildCommand func(config.Config, wiring.CLILLMOptions, string) (*cliinbound.Command, error),
	openAIBaseURL *string,
	openAIModel *string,
	openAIAPIKey *string,
	verbosity *int,
) *cobra.Command {
	var provider string
	var repo string
	var changeRequest string
	var base string
	var head string
	var comment bool
	var overview bool
	var suggest bool

	sub := &cobra.Command{
		Use:   "review",
		Short: "Run review and overview via GitHub context",
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts, logOverride, err := resolveLLMOptions(cmd, *openAIBaseURL, *openAIModel, *openAIAPIKey, *verbosity)
			if err != nil {
				return err
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

			startupLogger, err := wiring.BuildLogger(cfg, logOverride)
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

			cliCommand, err := buildCommand(cfg, opts, logOverride)
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

	flags := sub.Flags()
	flags.StringVar(&provider, "provider", "github", "provider name (only github is supported)")
	flags.StringVar(&repo, "repo", "", "repository (GitHub URL or owner/repo)")
	flags.StringVar(&changeRequest, "change-request", "", "GitHub pull request number")
	flags.StringVar(&base, "base", "", "base ref")
	flags.StringVar(&head, "head", "", "head ref or @staged/@all")
	flags.BoolVar(&comment, "comment", false, "post review result as pull request comments")
	flags.BoolVar(&overview, "overview", false, "generate and publish/print high-level overview output")
	flags.BoolVar(&suggest, "suggest", false, "enable suggested code changes in review findings")
	return sub
}

func newReplyCommentSubcommand(
	ctx context.Context,
	loadConfig func() (config.Config, error),
	buildCommand func(config.Config, wiring.CLILLMOptions, string) (*cliinbound.ReplyCommentCommand, error),
	openAIBaseURL *string,
	openAIModel *string,
	openAIAPIKey *string,
	verbosity *int,
) *cobra.Command {
	var provider string
	var repo string
	var changeRequest string
	var commentID string
	var question string
	var comment bool

	sub := &cobra.Command{
		Use:   "replycomment",
		Short: "Answer a PR comment question",
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts, logOverride, err := resolveLLMOptions(cmd, *openAIBaseURL, *openAIModel, *openAIAPIKey, *verbosity)
			if err != nil {
				return err
			}
			cfg, err := loadConfig()
			if err != nil {
				return cliConfigLoadError{cause: err}
			}

			startupLogger, err := wiring.BuildLogger(cfg, logOverride)
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

			cliCommand, err := buildCommand(cfg, opts, logOverride)
			if err != nil {
				return err
			}

			return cliCommand.Run(ctx, cliinbound.ReplyCommentRunParams{
				Provider:      provider,
				Repo:          repo,
				ChangeRequest: changeRequest,
				CommentID:     commentID,
				Question:      question,
				Comment:       comment,
			})
		},
	}

	flags := sub.Flags()
	flags.StringVar(&provider, "provider", "github", "provider name (only github is supported)")
	flags.StringVar(&repo, "repo", "", "repository (GitHub URL or owner/repo)")
	flags.StringVar(&changeRequest, "change-request", "", "GitHub pull request number")
	flags.StringVar(&commentID, "comment-id", "", "GitHub comment id to answer")
	flags.StringVar(&question, "question", "", "question text to answer")
	flags.BoolVar(&comment, "comment", false, "post reply as pull request comment (requires --comment-id)")
	return sub
}

func resolveLLMOptions(cmd *cobra.Command, openAIBaseURL string, openAIModel string, openAIAPIKey string, verbosity int) (wiring.CLILLMOptions, string, error) {
	opts := wiring.CLILLMOptions{}
	if flagChanged(cmd, "openai-base-url") {
		value, err := validateOpenAIStringFlagValue("openai-base-url", openAIBaseURL)
		if err != nil {
			return opts, "", err
		}
		opts.OpenAIBaseURL = value
	}
	if flagChanged(cmd, "openai-model") {
		value, err := validateOpenAIStringFlagValue("openai-model", openAIModel)
		if err != nil {
			return opts, "", err
		}
		opts.OpenAIModel = value
	}
	if flagChanged(cmd, "openai-api-key") {
		value, err := validateOpenAIStringFlagValue("openai-api-key", openAIAPIKey)
		if err != nil {
			return opts, "", err
		}
		opts.OpenAIAPIKey = value
	}
	logOverride := ""
	if flagChanged(cmd, "verbose") {
		logOverride = sharedcli.LogLevelOverrideFromVerbosity(verbosity)
	}
	return opts, logOverride, nil
}

func flagChanged(cmd *cobra.Command, name string) bool {
	if cmd.Flags().Changed(name) {
		return true
	}
	if cmd.InheritedFlags().Changed(name) {
		return true
	}
	return false
}

func validateOpenAIStringFlagValue(flagName string, value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || strings.HasPrefix(trimmed, "-") {
		return "", fmt.Errorf("flag --%s requires a non-empty value", flagName)
	}
	return trimmed, nil
}
