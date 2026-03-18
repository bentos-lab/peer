package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	cliinbound "bentos-backend/adapter/inbound/cli"
	"bentos-backend/config"
	"bentos-backend/wiring"
	"github.com/spf13/cobra"
)

// main bootstraps the CLI review command.
func main() {
	if err := runCLI(
		context.Background(),
		os.Args[1:],
		config.Load,
		func(cfg config.Config, opts wiring.CLILLMOptions) (*cliinbound.Command, error) {
			return wiring.BuildCLICommand(cfg, os.Stdout, opts)
		},
	); err != nil {
		log.Fatal(err)
	}
}

func runCLI(
	ctx context.Context,
	args []string,
	loadConfig func() (config.Config, error),
	buildCommand func(config.Config, wiring.CLILLMOptions) (*cliinbound.Command, error),
) error {
	root := newRootCommand(ctx, loadConfig, buildCommand)
	root.SetArgs(args)
	return root.ExecuteContext(ctx)
}

func newRootCommand(
	ctx context.Context,
	loadConfig func() (config.Config, error),
	buildCommand func(config.Config, wiring.CLILLMOptions) (*cliinbound.Command, error),
) *cobra.Command {
	var openAIBaseURL string
	var openAIModel string
	var openAIAPIKey string
	var changedFiles string
	var includeUnstaged bool
	var includeUntracked bool

	var cmd *cobra.Command
	cmd = &cobra.Command{
		Use:   "review",
		Short: "Run local repository review",
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

			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			cliCommand, err := buildCommand(cfg, opts)
			if err != nil {
				return err
			}

			return cliCommand.Run(ctx, cliinbound.RunParams{
				ChangedFiles:     changedFiles,
				IncludeUnstaged:  includeUnstaged,
				IncludeUntracked: includeUntracked,
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

	return cmd
}

func validateOpenAIStringFlagValue(flagName string, value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || strings.HasPrefix(trimmed, "-") {
		return "", fmt.Errorf("flag --%s requires a non-empty value", flagName)
	}
	return trimmed, nil
}
