package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"

	cliinbound "github.com/bentos-lab/peer/adapter/inbound/cli"
	gitlabinbound "github.com/bentos-lab/peer/adapter/inbound/http/gitlab"
	githubvcs "github.com/bentos-lab/peer/adapter/outbound/vcs/github"
	gitlabvcs "github.com/bentos-lab/peer/adapter/outbound/vcs/gitlab"
	"github.com/bentos-lab/peer/config"
	sharedcli "github.com/bentos-lab/peer/shared/cli"
	"github.com/bentos-lab/peer/shared/skillupdate"
	"github.com/bentos-lab/peer/usecase"
	"github.com/bentos-lab/peer/wiring"

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

type peerDeps struct {
	loadConfig               func() (config.Config, error)
	buildReviewCommand       func(config.Config, wiring.CLILLMOptions, string) (*cliinbound.ReviewCommand, error)
	buildOverviewCommand     func(config.Config, wiring.CLILLMOptions, string) (*cliinbound.OverviewCommand, error)
	buildAutogenCommand      func(config.Config, wiring.CLILLMOptions, string) (*cliinbound.AutogenCommand, error)
	buildReplyCommentCommand func(config.Config, wiring.CLILLMOptions, string) (*cliinbound.ReplyCommentCommand, error)
	buildGitHubHandler       func(config.Config) (http.Handler, error)
	buildGitLabHandler       func(config.Config) (http.Handler, *gitlabinbound.HookSyncer, error)
	listenAndServe           func(string, http.Handler) error
	resolveOriginURL         func() (string, error)
}

func defaultPeerDeps() peerDeps {
	return peerDeps{
		loadConfig: config.Load,
		buildReviewCommand: func(cfg config.Config, opts wiring.CLILLMOptions, logLevelOverride string) (*cliinbound.ReviewCommand, error) {
			deps, err := wiring.BuildCommonDependencies(cfg, opts, logLevelOverride)
			if err != nil {
				return nil, err
			}
			builder := func(repoURL string) (usecase.ReviewUseCase, error) {
				return wiring.BuildReviewUseCase(cfg, opts, logLevelOverride)
			}
			resolver := cliinbound.StaticVCSClients{
				GitHub: githubvcs.NewCLIClient(),
				GitLab: gitlabvcs.NewCLIClientWithConfig(gitlabvcs.CLIClientConfig{Host: opts.VCSHost}),
			}
			return cliinbound.NewReviewCommand(builder, resolver, deps.CodeEnvironmentFactory, deps.RecipeLoader, deps.Logger), nil
		},
		buildOverviewCommand: func(cfg config.Config, opts wiring.CLILLMOptions, logLevelOverride string) (*cliinbound.OverviewCommand, error) {
			deps, err := wiring.BuildCommonDependencies(cfg, opts, logLevelOverride)
			if err != nil {
				return nil, err
			}
			builder := func(repoURL string) (usecase.OverviewUseCase, error) {
				return wiring.BuildOverviewUseCase(cfg, opts, logLevelOverride)
			}
			resolver := cliinbound.StaticVCSClients{
				GitHub: githubvcs.NewCLIClient(),
				GitLab: gitlabvcs.NewCLIClientWithConfig(gitlabvcs.CLIClientConfig{Host: opts.VCSHost}),
			}
			return cliinbound.NewOverviewCommand(builder, resolver, deps.CodeEnvironmentFactory, deps.RecipeLoader, deps.Logger), nil
		},
		buildAutogenCommand: func(cfg config.Config, opts wiring.CLILLMOptions, logLevelOverride string) (*cliinbound.AutogenCommand, error) {
			deps, err := wiring.BuildCommonDependencies(cfg, opts, logLevelOverride)
			if err != nil {
				return nil, err
			}
			builder := func(repoURL string) (usecase.AutogenUseCase, error) {
				return wiring.BuildAutogenUseCase(cfg, opts, logLevelOverride)
			}
			resolver := cliinbound.StaticVCSClients{
				GitHub: githubvcs.NewCLIClient(),
				GitLab: gitlabvcs.NewCLIClientWithConfig(gitlabvcs.CLIClientConfig{Host: opts.VCSHost}),
			}
			return cliinbound.NewAutogenCommand(builder, resolver, deps.CodeEnvironmentFactory, deps.RecipeLoader, deps.Logger), nil
		},
		buildReplyCommentCommand: func(cfg config.Config, opts wiring.CLILLMOptions, logLevelOverride string) (*cliinbound.ReplyCommentCommand, error) {
			builder := func(repoURL string) (usecase.ReplyCommentUseCase, error) {
				return wiring.BuildReplyCommentUseCase(cfg, opts, logLevelOverride)
			}
			deps, err := wiring.BuildCommonDependencies(cfg, opts, logLevelOverride)
			if err != nil {
				return nil, err
			}
			resolver := cliinbound.StaticVCSClients{
				GitHub: githubvcs.NewCLIClient(),
				GitLab: gitlabvcs.NewCLIClientWithConfig(gitlabvcs.CLIClientConfig{Host: opts.VCSHost}),
			}
			return cliinbound.NewReplyCommentCommand(builder, resolver, deps.CodeEnvironmentFactory, deps.RecipeLoader, cfg.ReplyComment.TriggerName, deps.Logger), nil
		},
		buildGitHubHandler: func(cfg config.Config) (http.Handler, error) {
			return wiring.BuildGitHubHandler(cfg)
		},
		buildGitLabHandler: func(cfg config.Config) (http.Handler, *gitlabinbound.HookSyncer, error) {
			return wiring.BuildGitLabHandler(cfg)
		},
		listenAndServe: http.ListenAndServe,
		resolveOriginURL: func() (string, error) {
			output, err := exec.Command("git", "config", "--get", "remote.origin.url").CombinedOutput()
			if err != nil {
				message := strings.TrimSpace(string(output))
				if message == "" {
					return "", fmt.Errorf("failed to resolve remote.origin.url: %w", err)
				}
				return "", fmt.Errorf("failed to resolve remote.origin.url: %w: %s", err, message)
			}
			originURL := strings.TrimSpace(string(output))
			if originURL == "" {
				return "", fmt.Errorf("missing remote.origin.url in local workspace")
			}
			return originURL, nil
		},
	}
}

func newRootCommand(ctx context.Context, deps peerDeps, version string, commit string) *cobra.Command {
	var cmd *cobra.Command
	cmd = &cobra.Command{
		Use:   "peer",
		Short: "Run repository review via GitHub context",
		RunE: func(_ *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.Version = fmt.Sprintf("version: %s\ncommit: %s", version, commit)
	cmd.SetVersionTemplate("{{.Version}}\n")

	cmd.AddCommand(newReviewSubcommand(ctx, deps.loadConfig, deps.buildReviewCommand, deps.resolveOriginURL))
	cmd.AddCommand(newOverviewSubcommand(ctx, deps.loadConfig, deps.buildOverviewCommand, deps.resolveOriginURL))
	cmd.AddCommand(newAutogenSubcommand(ctx, deps.loadConfig, deps.buildAutogenCommand, deps.resolveOriginURL))
	cmd.AddCommand(newReplyCommentSubcommand(ctx, deps.loadConfig, deps.buildReplyCommentCommand, deps.resolveOriginURL))
	cmd.AddCommand(newInstallSubcommand(ctx))
	cmd.AddCommand(newUpdateSubcommand(ctx, version))
	cmd.AddCommand(newWebhookSubcommand(ctx, deps))

	return cmd
}

func newReviewSubcommand(
	ctx context.Context,
	loadConfig func() (config.Config, error),
	buildCommand func(config.Config, wiring.CLILLMOptions, string) (*cliinbound.ReviewCommand, error),
	resolveOriginURL func() (string, error),
) *cobra.Command {
	var vcsProvider string
	var repo string
	var changeRequest string
	var base string
	var head string
	var publish bool
	var suggest bool
	var llmOpenAIBaseURL string
	var llmOpenAIModel string
	var llmOpenAIAPIKey string
	var codeAgent string
	var codeAgentProvider string
	var codeAgentModel string
	var verbosity int

	sub := &cobra.Command{
		Use:   "review",
		Short: "Run review via GitHub context",
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts, logOverride, err := resolveLLMOptions(cmd, llmOpenAIBaseURL, llmOpenAIModel, llmOpenAIAPIKey, codeAgent, codeAgentProvider, codeAgentModel, verbosity)
			if err != nil {
				return err
			}
			parsedProvider, vcsHost, err := resolveCLIProvider(cmd, vcsProvider, repo, resolveOriginURL)
			if err != nil {
				return err
			}
			opts.VCSProvider = parsedProvider
			opts.VCSHost = vcsHost
			cfg, err := loadConfig()
			if err != nil {
				return cliConfigLoadError{cause: err}
			}
			configOverrides, err := resolveConfigOverrides(cmd, llmOpenAIBaseURL, llmOpenAIModel, llmOpenAIAPIKey, codeAgent, codeAgentProvider, codeAgentModel)
			if err != nil {
				return err
			}
			cfg = sharedcli.ApplyConfigOverrides(cfg, configOverrides)

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

			return cliCommand.Run(ctx, cfg, cliinbound.ReviewParams{
				VCSProvider:   parsedProvider,
				VCSHost:       vcsHost,
				Repo:          repo,
				ChangeRequest: changeRequest,
				Base:          base,
				Head:          head,
				Publish:       publish,
				Suggest:       boolPointerIfChanged(cmd, "suggest", suggest),
			})
		},
	}

	flags := sub.Flags()
	flags.StringVar(&vcsProvider, "vcs-provider", "", sharedcli.VCSProviderFlagHelp())
	flags.StringVar(&repo, "repo", "", "repository (URL or owner/repo)")
	flags.StringVar(&changeRequest, "change-request", "", "pull request number")
	flags.StringVar(&base, "base", "", "base ref")
	flags.StringVar(&head, "head", "", "head ref or @staged/@all")
	flags.BoolVar(&publish, "publish", false, "post review result as pull request comments")
	flags.BoolVar(&suggest, "suggest", false, "enable suggested code changes in review findings")
	flags.StringVar(&llmOpenAIBaseURL, "llm-openai-base-url", "", "OpenAI compatible base URL override (empty to use coding-agent LLM, env: LLM_OPENAI_BASE_URL)")
	flags.StringVar(&llmOpenAIModel, "llm-openai-model", "", "OpenAI compatible model override (env: LLM_OPENAI_MODEL)")
	flags.StringVar(&llmOpenAIAPIKey, "llm-openai-api-key", "", "OpenAI compatible API key override (env: LLM_OPENAI_API_KEY)")
	flags.StringVar(&codeAgent, "code-agent", "", "coding agent override (empty to use config, env: CODING_AGENT_NAME)")
	flags.StringVar(&codeAgentProvider, "code-agent-provider", "", "coding agent provider override (empty to use config, env: CODING_AGENT_PROVIDER)")
	flags.StringVar(&codeAgentModel, "code-agent-model", "", "coding agent model override (empty to use config, env: CODING_AGENT_MODEL)")
	flags.CountVarP(&verbosity, "verbose", "v", "increase log verbosity (-v=debug, -vv=trace, default=info)")
	return sub
}

func newOverviewSubcommand(
	ctx context.Context,
	loadConfig func() (config.Config, error),
	buildCommand func(config.Config, wiring.CLILLMOptions, string) (*cliinbound.OverviewCommand, error),
	resolveOriginURL func() (string, error),
) *cobra.Command {
	var vcsProvider string
	var repo string
	var changeRequest string
	var base string
	var head string
	var publish bool
	var issueAlignment bool
	var llmOpenAIBaseURL string
	var llmOpenAIModel string
	var llmOpenAIAPIKey string
	var codeAgent string
	var codeAgentProvider string
	var codeAgentModel string
	var verbosity int

	sub := &cobra.Command{
		Use:   "overview",
		Short: "Run overview via GitHub context",
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts, logOverride, err := resolveLLMOptions(cmd, llmOpenAIBaseURL, llmOpenAIModel, llmOpenAIAPIKey, codeAgent, codeAgentProvider, codeAgentModel, verbosity)
			if err != nil {
				return err
			}
			parsedProvider, vcsHost, err := resolveCLIProvider(cmd, vcsProvider, repo, resolveOriginURL)
			if err != nil {
				return err
			}
			opts.VCSProvider = parsedProvider
			opts.VCSHost = vcsHost
			cfg, err := loadConfig()
			if err != nil {
				return cliConfigLoadError{cause: err}
			}
			configOverrides, err := resolveConfigOverrides(cmd, llmOpenAIBaseURL, llmOpenAIModel, llmOpenAIAPIKey, codeAgent, codeAgentProvider, codeAgentModel)
			if err != nil {
				return err
			}
			cfg = sharedcli.ApplyConfigOverrides(cfg, configOverrides)

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

			return cliCommand.Run(ctx, cfg, cliinbound.OverviewParams{
				VCSProvider:    parsedProvider,
				VCSHost:        vcsHost,
				Repo:           repo,
				ChangeRequest:  changeRequest,
				Base:           base,
				Head:           head,
				Publish:        publish,
				IssueAlignment: boolPointerIfChanged(cmd, "issue-alignment", issueAlignment),
			})
		},
	}

	flags := sub.Flags()
	flags.StringVar(&vcsProvider, "vcs-provider", "", sharedcli.VCSProviderFlagHelp())
	flags.StringVar(&repo, "repo", "", "repository (URL or owner/repo)")
	flags.StringVar(&changeRequest, "change-request", "", "pull request number")
	flags.StringVar(&base, "base", "", "base ref")
	flags.StringVar(&head, "head", "", "head ref or @staged/@all")
	flags.BoolVar(&publish, "publish", false, "post overview result as pull request comments")
	flags.BoolVar(&issueAlignment, "issue-alignment", false, "enable issue alignment analysis for overview")
	flags.StringVar(&llmOpenAIBaseURL, "llm-openai-base-url", "", "OpenAI compatible base URL override (empty to use coding-agent LLM, env: LLM_OPENAI_BASE_URL)")
	flags.StringVar(&llmOpenAIModel, "llm-openai-model", "", "OpenAI compatible model override (env: LLM_OPENAI_MODEL)")
	flags.StringVar(&llmOpenAIAPIKey, "llm-openai-api-key", "", "OpenAI compatible API key override (env: LLM_OPENAI_API_KEY)")
	flags.StringVar(&codeAgent, "code-agent", "", "coding agent override (empty to use config, env: CODING_AGENT_NAME)")
	flags.StringVar(&codeAgentProvider, "code-agent-provider", "", "coding agent provider override (empty to use config, env: CODING_AGENT_PROVIDER)")
	flags.StringVar(&codeAgentModel, "code-agent-model", "", "coding agent model override (empty to use config, env: CODING_AGENT_MODEL)")
	flags.CountVarP(&verbosity, "verbose", "v", "increase log verbosity (-v=debug, -vv=trace, default=info)")
	return sub
}

func newAutogenSubcommand(
	ctx context.Context,
	loadConfig func() (config.Config, error),
	buildCommand func(config.Config, wiring.CLILLMOptions, string) (*cliinbound.AutogenCommand, error),
	resolveOriginURL func() (string, error),
) *cobra.Command {
	var vcsProvider string
	var repo string
	var changeRequest string
	var base string
	var head string
	var publish bool
	var docs bool
	var tests bool
	var llmOpenAIBaseURL string
	var llmOpenAIModel string
	var llmOpenAIAPIKey string
	var codeAgent string
	var codeAgentProvider string
	var codeAgentModel string
	var verbosity int

	sub := &cobra.Command{
		Use:   "autogen",
		Short: "Run autogen (lazywork) for tests/docs/comments",
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts, logOverride, err := resolveLLMOptions(cmd, llmOpenAIBaseURL, llmOpenAIModel, llmOpenAIAPIKey, codeAgent, codeAgentProvider, codeAgentModel, verbosity)
			if err != nil {
				return err
			}
			parsedProvider, vcsHost, err := resolveCLIProvider(cmd, vcsProvider, repo, resolveOriginURL)
			if err != nil {
				return err
			}
			opts.VCSProvider = parsedProvider
			opts.VCSHost = vcsHost
			cfg, err := loadConfig()
			if err != nil {
				return cliConfigLoadError{cause: err}
			}
			configOverrides, err := resolveConfigOverrides(cmd, llmOpenAIBaseURL, llmOpenAIModel, llmOpenAIAPIKey, codeAgent, codeAgentProvider, codeAgentModel)
			if err != nil {
				return err
			}
			cfg = sharedcli.ApplyConfigOverrides(cfg, configOverrides)

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

			return cliCommand.Run(ctx, cfg, cliinbound.AutogenRunParams{
				VCSProvider:   parsedProvider,
				VCSHost:       vcsHost,
				Repo:          repo,
				ChangeRequest: changeRequest,
				Base:          base,
				Head:          head,
				Publish:       publish,
				Docs:          boolPointerIfChanged(cmd, "docs", docs),
				Tests:         boolPointerIfChanged(cmd, "tests", tests),
			})
		},
	}

	flags := sub.Flags()
	flags.StringVar(&vcsProvider, "vcs-provider", "", sharedcli.VCSProviderFlagHelp())
	flags.StringVar(&repo, "repo", "", "repository (URL or owner/repo)")
	flags.StringVar(&changeRequest, "change-request", "", "pull request number")
	flags.StringVar(&base, "base", "", "base ref")
	flags.StringVar(&head, "head", "", "head ref or @staged/@all")
	flags.BoolVar(&publish, "publish", false, "post autogen summary and push changes to PR branch")
	flags.BoolVar(&docs, "docs", false, "generate docs and code comments")
	flags.BoolVar(&tests, "tests", false, "generate tests")
	flags.StringVar(&llmOpenAIBaseURL, "llm-openai-base-url", "", "OpenAI compatible base URL override (empty to use coding-agent LLM, env: LLM_OPENAI_BASE_URL)")
	flags.StringVar(&llmOpenAIModel, "llm-openai-model", "", "OpenAI compatible model override (env: LLM_OPENAI_MODEL)")
	flags.StringVar(&llmOpenAIAPIKey, "llm-openai-api-key", "", "OpenAI compatible API key override (env: LLM_OPENAI_API_KEY)")
	flags.StringVar(&codeAgent, "code-agent", "", "coding agent override (empty to use config, env: CODING_AGENT_NAME)")
	flags.StringVar(&codeAgentProvider, "code-agent-provider", "", "coding agent provider override (empty to use config, env: CODING_AGENT_PROVIDER)")
	flags.StringVar(&codeAgentModel, "code-agent-model", "", "coding agent model override (empty to use config, env: CODING_AGENT_MODEL)")
	flags.CountVarP(&verbosity, "verbose", "v", "increase log verbosity (-v=debug, -vv=trace, default=info)")
	return sub
}

func newReplyCommentSubcommand(
	ctx context.Context,
	loadConfig func() (config.Config, error),
	buildCommand func(config.Config, wiring.CLILLMOptions, string) (*cliinbound.ReplyCommentCommand, error),
	resolveOriginURL func() (string, error),
) *cobra.Command {
	var vcsProvider string
	var repo string
	var changeRequest string
	var commentID string
	var question string
	var publish bool
	var llmOpenAIBaseURL string
	var llmOpenAIModel string
	var llmOpenAIAPIKey string
	var codeAgent string
	var codeAgentProvider string
	var codeAgentModel string
	var verbosity int

	sub := &cobra.Command{
		Use:   "replycomment",
		Short: "Answer a PR comment question",
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts, logOverride, err := resolveLLMOptions(cmd, llmOpenAIBaseURL, llmOpenAIModel, llmOpenAIAPIKey, codeAgent, codeAgentProvider, codeAgentModel, verbosity)
			if err != nil {
				return err
			}
			parsedProvider, vcsHost, err := resolveCLIProvider(cmd, vcsProvider, repo, resolveOriginURL)
			if err != nil {
				return err
			}
			opts.VCSProvider = parsedProvider
			opts.VCSHost = vcsHost
			cfg, err := loadConfig()
			if err != nil {
				return cliConfigLoadError{cause: err}
			}
			configOverrides, err := resolveConfigOverrides(cmd, llmOpenAIBaseURL, llmOpenAIModel, llmOpenAIAPIKey, codeAgent, codeAgentProvider, codeAgentModel)
			if err != nil {
				return err
			}
			cfg = sharedcli.ApplyConfigOverrides(cfg, configOverrides)

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

			return cliCommand.Run(ctx, cfg, cliinbound.ReplyCommentRunParams{
				VCSProvider:   parsedProvider,
				VCSHost:       vcsHost,
				Repo:          repo,
				ChangeRequest: changeRequest,
				CommentID:     commentID,
				Question:      question,
				Publish:       publish,
			})
		},
	}

	flags := sub.Flags()
	flags.StringVar(&vcsProvider, "vcs-provider", "", sharedcli.VCSProviderFlagHelp())
	flags.StringVar(&repo, "repo", "", "repository (URL or owner/repo)")
	flags.StringVar(&changeRequest, "change-request", "", "pull request number")
	flags.StringVar(&commentID, "comment-id", "", "comment id to answer")
	flags.StringVar(&question, "question", "", "question text to answer")
	flags.BoolVar(&publish, "publish", false, "post reply as pull request comment (requires --comment-id)")
	flags.StringVar(&llmOpenAIBaseURL, "llm-openai-base-url", "", "OpenAI compatible base URL override (empty to use coding-agent LLM, env: LLM_OPENAI_BASE_URL)")
	flags.StringVar(&llmOpenAIModel, "llm-openai-model", "", "OpenAI compatible model override (env: LLM_OPENAI_MODEL)")
	flags.StringVar(&llmOpenAIAPIKey, "llm-openai-api-key", "", "OpenAI compatible API key override (env: LLM_OPENAI_API_KEY)")
	flags.StringVar(&codeAgent, "code-agent", "", "coding agent override (empty to use config, env: CODING_AGENT_NAME)")
	flags.StringVar(&codeAgentProvider, "code-agent-provider", "", "coding agent provider override (empty to use config, env: CODING_AGENT_PROVIDER)")
	flags.StringVar(&codeAgentModel, "code-agent-model", "", "coding agent model override (empty to use config, env: CODING_AGENT_MODEL)")
	flags.CountVarP(&verbosity, "verbose", "v", "increase log verbosity (-v=debug, -vv=trace, default=info)")
	return sub
}

func newInstallSubcommand(ctx context.Context) *cobra.Command {
	command := cliinbound.NewInstallCommand()

	sub := &cobra.Command{
		Use:   "install",
		Short: "Install required CLI dependencies",
	}

	ghCmd := &cobra.Command{
		Use:   "gh",
		Short: "Install GitHub CLI (gh)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			login, err := cmd.Flags().GetBool("login")
			if err != nil {
				return err
			}
			outcome, err := command.InstallGh(ctx, login)
			if err != nil {
				return err
			}
			printInstallOutcome(cmd.OutOrStdout(), "gh", outcome)
			return nil
		},
	}
	ghCmd.Flags().Bool("login", false, "run `gh auth login` after install")

	glabCmd := &cobra.Command{
		Use:   "glab",
		Short: "Install GitLab CLI (glab)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			login, err := cmd.Flags().GetBool("login")
			if err != nil {
				return err
			}
			outcome, err := command.InstallGlab(ctx, login)
			if err != nil {
				return err
			}
			printInstallOutcome(cmd.OutOrStdout(), "glab", outcome)
			return nil
		},
	}
	glabCmd.Flags().Bool("login", false, "run `glab auth login` after install")

	opencodeCmd := &cobra.Command{
		Use:   "opencode",
		Short: "Install OpenCode (opencode)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			outcome, err := command.InstallOpencode(ctx)
			if err != nil {
				return err
			}
			printInstallOutcome(cmd.OutOrStdout(), "opencode", outcome)
			return nil
		},
	}

	gitCmd := &cobra.Command{
		Use:   "git",
		Short: "Install Git",
		RunE: func(cmd *cobra.Command, _ []string) error {
			outcome, err := command.InstallGit(ctx)
			if err != nil {
				return err
			}
			printInstallOutcome(cmd.OutOrStdout(), "git", outcome)
			return nil
		},
	}

	quickstartCmd := &cobra.Command{
		Use:   "quickstart",
		Short: "Install gh (with login) and opencode",
		RunE: func(cmd *cobra.Command, _ []string) error {
			outcome, err := command.InstallQuickstart(ctx)
			if err != nil {
				return err
			}
			printInstallOutcome(cmd.OutOrStdout(), "gh", outcome.Gh)
			printInstallOutcome(cmd.OutOrStdout(), "opencode", outcome.Opencode)
			return nil
		},
	}

	sub.AddCommand(ghCmd)
	sub.AddCommand(glabCmd)
	sub.AddCommand(opencodeCmd)
	sub.AddCommand(gitCmd)
	sub.AddCommand(quickstartCmd)
	return sub
}

func newUpdateSubcommand(ctx context.Context, currentVersion string) *cobra.Command {
	sub := &cobra.Command{
		Use:   "update",
		Short: "Update peer components",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	sub.AddCommand(newUpdateSelfSubcommand(ctx, cliinbound.NewUpdateCommand(currentVersion)))
	sub.AddCommand(newUpdateSkillsSubcommand(ctx, cliinbound.NewSkillUpdateCommand()))
	return sub
}

type skillUpdateRunner interface {
	Run(context.Context, []string) ([]skillupdate.Result, error)
}

func newUpdateSelfSubcommand(ctx context.Context, command *cliinbound.UpdateCommand) *cobra.Command {
	sub := &cobra.Command{
		Use:   "self",
		Short: "Update peer to the latest stable release",
		RunE: func(cmd *cobra.Command, _ []string) error {
			result, err := command.Run(ctx)
			if err != nil {
				return err
			}
			if result.UpToDate {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Up-to-date.")
				return nil
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Updated peer to %s\n", result.Result.Version)
			return nil
		},
	}
	return sub
}

func printInstallOutcome(out io.Writer, tool string, outcome cliinbound.InstallOutcome) {
	if outcome.Installed {
		_, _ = fmt.Fprintf(out, "Installed %s.\n", tool)
	} else {
		_, _ = fmt.Fprintf(out, "Already installed: %s.\n", tool)
	}
	if outcome.AlreadyAuthenticated {
		_, _ = fmt.Fprintf(out, "%s already authenticated. Skipping login.\n", tool)
	}
}

func newUpdateSkillsSubcommand(ctx context.Context, command skillUpdateRunner) *cobra.Command {
	var paths []string
	sub := &cobra.Command{
		Use:   "skills",
		Short: "Update peer skills in agent directories",
		RunE: func(cmd *cobra.Command, _ []string) error {
			results, err := command.Run(ctx, paths)
			for _, result := range results {
				if result.Err != nil {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s error: %v\n", result.Path, result.Err)
					continue
				}
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s ok\n", result.Path)
			}
			return err
		},
	}
	sub.Flags().StringArrayVar(&paths, "path", nil, "path to a skills root or peer skill directory (repeatable)")
	return sub
}

func resolveLLMOptions(cmd *cobra.Command, llmOpenAIBaseURL string, llmOpenAIModel string, llmOpenAIAPIKey string, codeAgent string, codeAgentProvider string, codeAgentModel string, verbosity int) (wiring.CLILLMOptions, string, error) {
	opts := wiring.CLILLMOptions{}
	opts.ForceCLIPublishers = true
	if flagChanged(cmd, "llm-openai-base-url") {
		opts.OpenAIBaseURLSet = true
		value := strings.TrimSpace(llmOpenAIBaseURL)
		if value != "" && strings.HasPrefix(value, "-") {
			return opts, "", fmt.Errorf("flag --llm-openai-base-url requires a non-empty value or empty override")
		}
		opts.OpenAIBaseURL = value
	}
	if flagChanged(cmd, "llm-openai-model") {
		value := validateOpenAIStringFlagValue(llmOpenAIModel)
		if value == "" {
			return opts, "", fmt.Errorf("flag --llm-openai-model requires a non-empty value")
		}
		opts.OpenAIModel = value
	}
	if flagChanged(cmd, "llm-openai-api-key") {
		value := validateOpenAIStringFlagValue(llmOpenAIAPIKey)
		if value == "" {
			return opts, "", fmt.Errorf("flag --llm-openai-api-key requires a non-empty value")
		}
		opts.OpenAIAPIKey = value
		opts.OpenAIAPIKeySet = true
	}
	if flagChanged(cmd, "code-agent") {
		value := validateNonEmptyOverrideFlag(codeAgent)
		if value == "" {
			return opts, "", fmt.Errorf("flag --code-agent requires a non-empty value")
		}
		opts.CodeAgent = value
		opts.CodeAgentSet = true
	}
	if flagChanged(cmd, "code-agent-provider") {
		value := validateNonEmptyOverrideFlag(codeAgentProvider)
		if value == "" {
			return opts, "", fmt.Errorf("flag --code-agent-provider requires a non-empty value")
		}
		opts.CodeAgentProvider = value
		opts.CodeAgentProviderSet = true
	}
	if flagChanged(cmd, "code-agent-model") {
		value := validateNonEmptyOverrideFlag(codeAgentModel)
		if value == "" {
			return opts, "", fmt.Errorf("flag --code-agent-model requires a non-empty value")
		}
		opts.CodeAgentModel = value
		opts.CodeAgentModelSet = true
	}
	logOverride := sharedcli.LogLevelOverrideFromVerbosity(verbosity)
	return opts, logOverride, nil
}

func resolveConfigOverrides(cmd *cobra.Command, llmOpenAIBaseURL string, llmOpenAIModel string, llmOpenAIAPIKey string, codeAgent string, codeAgentProvider string, codeAgentModel string) (sharedcli.ConfigOverrides, error) {
	overrides := sharedcli.ConfigOverrides{}
	if flagChanged(cmd, "llm-openai-base-url") {
		value := strings.TrimSpace(llmOpenAIBaseURL)
		if value != "" && strings.HasPrefix(value, "-") {
			return overrides, fmt.Errorf("flag --llm-openai-base-url requires a non-empty value or empty override")
		}
		overrides.OpenAIBaseURL = &value
	}
	if flagChanged(cmd, "llm-openai-model") {
		value := validateOpenAIStringFlagValue(llmOpenAIModel)
		if value == "" {
			return overrides, fmt.Errorf("flag --llm-openai-model requires a non-empty value")
		}
		overrides.OpenAIModel = &value
	}
	if flagChanged(cmd, "llm-openai-api-key") {
		value := validateOpenAIStringFlagValue(llmOpenAIAPIKey)
		if value == "" {
			return overrides, fmt.Errorf("flag --llm-openai-api-key requires a non-empty value")
		}
		overrides.OpenAIAPIKey = &value
	}
	if flagChanged(cmd, "code-agent") {
		value := validateNonEmptyOverrideFlag(codeAgent)
		if value == "" {
			return overrides, fmt.Errorf("flag --code-agent requires a non-empty value")
		}
		overrides.CodingAgentName = &value
	}
	if flagChanged(cmd, "code-agent-provider") {
		value := validateNonEmptyOverrideFlag(codeAgentProvider)
		if value == "" {
			return overrides, fmt.Errorf("flag --code-agent-provider requires a non-empty value")
		}
		overrides.CodingAgentProvider = &value
	}
	if flagChanged(cmd, "code-agent-model") {
		value := validateNonEmptyOverrideFlag(codeAgentModel)
		if value == "" {
			return overrides, fmt.Errorf("flag --code-agent-model requires a non-empty value")
		}
		overrides.CodingAgentModel = &value
	}
	return overrides, nil
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

func boolPointerIfChanged(cmd *cobra.Command, name string, value bool) *bool {
	if !cmd.Flags().Changed(name) {
		return nil
	}
	return &value
}

func validateOpenAIStringFlagValue(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || strings.HasPrefix(trimmed, "-") {
		return ""
	}
	return trimmed
}

func validateNonEmptyOverrideFlag(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || strings.HasPrefix(trimmed, "-") {
		return ""
	}
	return trimmed
}

func resolveCLIProvider(cmd *cobra.Command, rawProvider string, repo string, resolveOriginURL func() (string, error)) (string, string, error) {
	if flagChanged(cmd, "vcs-provider") {
		value := strings.TrimSpace(rawProvider)
		if value == "" {
			return "", "", fmt.Errorf("flag --vcs-provider requires a non-empty value (supported: %s)", sharedcli.SupportedVCSProviderValuesText())
		}
		return sharedcli.ParseVCSProvider(value)
	}

	if strings.TrimSpace(repo) != "" {
		provider, host, err := sharedcli.ResolveVCSProviderFromRepo(repo)
		if err != nil {
			return "", "", fmt.Errorf("unable to auto-detect vcs provider from --repo: %w; please set --vcs-provider (supported: %s)", err, sharedcli.SupportedVCSProviderValuesText())
		}
		return provider, host, nil
	}

	if resolveOriginURL == nil {
		return "", "", fmt.Errorf("unable to auto-detect vcs provider; please set --vcs-provider (supported: %s)", sharedcli.SupportedVCSProviderValuesText())
	}
	originURL, err := resolveOriginURL()
	if err != nil {
		return "", "", fmt.Errorf("unable to auto-detect vcs provider from local repository: %w; please set --vcs-provider (supported: %s)", err, sharedcli.SupportedVCSProviderValuesText())
	}
	provider, host, err := sharedcli.ResolveVCSProviderFromRepo(originURL)
	if err != nil {
		return "", "", fmt.Errorf("unable to auto-detect vcs provider from local repository: %w; please set --vcs-provider (supported: %s)", err, sharedcli.SupportedVCSProviderValuesText())
	}
	return provider, host, nil
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
	codingAgentConfig := wiring.ResolveCLICodingAgentConfig(cfg, opts)
	logger.Infof(
		`cli startup: llm=codingagent agent=%q provider=%q model=%q`,
		codingAgentConfig.Agent,
		codingAgentConfig.Provider,
		codingAgentConfig.Model,
	)
	return nil
}
