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
	"bentos-backend/usecase"
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
		func(cfg config.Config, opts wiring.CLILLMOptions, logLevelOverride string) (*cliinbound.AutogenCommand, error) {
			return wiring.BuildCLIAutogenCommand(cfg, opts, logLevelOverride)
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
	buildAutogenCommand func(config.Config, wiring.CLILLMOptions, string) (*cliinbound.AutogenCommand, error),
	buildReplyCommentCommand func(config.Config, wiring.CLILLMOptions, string) (*cliinbound.ReplyCommentCommand, error),
) error {
	root := newRootCommand(ctx, loadConfig, buildReviewCommand, buildAutogenCommand, buildReplyCommentCommand)
	root.SetArgs(args)
	return root.ExecuteContext(ctx)
}

func newRootCommand(
	ctx context.Context,
	loadConfig func() (config.Config, error),
	buildReviewCommand func(config.Config, wiring.CLILLMOptions, string) (*cliinbound.Command, error),
	buildAutogenCommand func(config.Config, wiring.CLILLMOptions, string) (*cliinbound.AutogenCommand, error),
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
	persistentFlags.StringVar(&openAIBaseURL, "openai-base-url", "", "OpenAI compatible base URL override (empty to use coding-agent LLM)")
	persistentFlags.StringVar(&openAIModel, "openai-model", "", "OpenAI compatible model override")
	persistentFlags.StringVar(&openAIAPIKey, "openai-api-key", "", "OpenAI compatible API key override")
	persistentFlags.CountVarP(&verbosity, "verbose", "v", "increase log verbosity (-v=debug, -vv=trace, default=info)")

	cmd.AddCommand(newReviewSubcommand(ctx, loadConfig, buildReviewCommand, &openAIBaseURL, &openAIModel, &openAIAPIKey, &verbosity))
	cmd.AddCommand(newAutogenSubcommand(ctx, loadConfig, buildAutogenCommand, &openAIBaseURL, &openAIModel, &openAIAPIKey, &verbosity))
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
			if err := logLLMSelection(startupLogger, cfg, opts); err != nil {
				return err
			}

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

func newAutogenSubcommand(
	ctx context.Context,
	loadConfig func() (config.Config, error),
	buildCommand func(config.Config, wiring.CLILLMOptions, string) (*cliinbound.AutogenCommand, error),
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
	var publish bool
	var docs bool
	var tests bool

	sub := &cobra.Command{
		Use:   "autogen",
		Short: "Run autogen (lazywork) for tests/docs/comments",
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
			if err := logLLMSelection(startupLogger, cfg, opts); err != nil {
				return err
			}

			cliCommand, err := buildCommand(cfg, opts, logOverride)
			if err != nil {
				return err
			}

			return cliCommand.Run(ctx, cliinbound.AutogenRunParams{
				Provider:      provider,
				Repo:          repo,
				ChangeRequest: changeRequest,
				Base:          base,
				Head:          head,
				Publish:       publish,
				Docs:          docs,
				Tests:         tests,
			})
		},
	}

	flags := sub.Flags()
	flags.StringVar(&provider, "provider", "github", "provider name (only github is supported)")
	flags.StringVar(&repo, "repo", "", "repository (GitHub URL or owner/repo)")
	flags.StringVar(&changeRequest, "change-request", "", "GitHub pull request number")
	flags.StringVar(&base, "base", "", "base ref")
	flags.StringVar(&head, "head", "", "head ref or @staged/@all")
	flags.BoolVar(&publish, "publish", false, "post autogen summary and push changes to PR branch")
	flags.BoolVar(&docs, "docs", false, "generate docs and code comments")
	flags.BoolVar(&tests, "tests", false, "generate tests")
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
			if err := logLLMSelection(startupLogger, cfg, opts); err != nil {
				return err
			}

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
		opts.OpenAIBaseURLSet = true
		value := strings.TrimSpace(openAIBaseURL)
		if value != "" && strings.HasPrefix(value, "-") {
			return opts, "", fmt.Errorf("flag --openai-base-url requires a non-empty value or empty override")
		}
		opts.OpenAIBaseURL = value
	}
	if flagChanged(cmd, "openai-model") {
		value := validateOpenAIStringFlagValue("openai-model", openAIModel)
		if value == "" {
			return opts, "", fmt.Errorf("flag --openai-model requires a non-empty value")
		}
		opts.OpenAIModel = value
	}
	if flagChanged(cmd, "openai-api-key") {
		value := validateOpenAIStringFlagValue("openai-api-key", openAIAPIKey)
		if value == "" {
			return opts, "", fmt.Errorf("flag --openai-api-key requires a non-empty value")
		}
		opts.OpenAIAPIKey = value
		opts.OpenAIAPIKeySet = true
	}
	logOverride := sharedcli.LogLevelOverrideFromVerbosity(verbosity)
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

func validateOpenAIStringFlagValue(flagName string, value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || strings.HasPrefix(trimmed, "-") {
		return ""
	}
	return trimmed
}

func logLLMSelection(logger usecase.Logger, cfg config.Config, opts wiring.CLILLMOptions) error {
	selection, err := wiring.ResolveLLMSelection(cfg, opts)
	if err != nil {
		return err
	}
	if selection.UseOpenAI {
		logger.Infof(
			`cli startup: llm=openai base_url=%q model=%q`,
			selection.OpenAI.BaseURL,
			selection.OpenAI.Model,
		)
		return nil
	}
	logger.Infof(
		`cli startup: llm=codingagent agent=%q provider=%q model=%q`,
		cfg.CodingAgent.Agent,
		cfg.CodingAgent.Provider,
		cfg.CodingAgent.Model,
	)
	return nil
}
