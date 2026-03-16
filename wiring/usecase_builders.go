package wiring

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	autogencodingagent "bentos-backend/adapter/outbound/autogen/codingagent"
	codeenvhost "bentos-backend/adapter/outbound/codeenv/host"
	customrecipe "bentos-backend/adapter/outbound/customrecipe"
	issuealignment "bentos-backend/adapter/outbound/issuealignment/codeagent"
	llmtracing "bentos-backend/adapter/outbound/llm/tracing"
	overviewcodingagent "bentos-backend/adapter/outbound/overview/codingagent"
	clipublisher "bentos-backend/adapter/outbound/publisher/cli"
	githubpublisher "bentos-backend/adapter/outbound/publisher/github"
	gitlabpublisher "bentos-backend/adapter/outbound/publisher/gitlab"
	routerpublisher "bentos-backend/adapter/outbound/publisher/router"
	replycommentcodingagent "bentos-backend/adapter/outbound/replycomment/codingagent"
	reviewercodingagent "bentos-backend/adapter/outbound/reviewer/codingagent"
	safetysanitizer "bentos-backend/adapter/outbound/safetysanitizer"
	githubvcs "bentos-backend/adapter/outbound/vcs/github"
	gitlabvcs "bentos-backend/adapter/outbound/vcs/gitlab"
	"bentos-backend/config"
	"bentos-backend/usecase"
	"bentos-backend/usecase/contracts"
	"bentos-backend/usecase/rulepack"
)

// BuildReviewUseCase builds a review usecase.
func BuildReviewUseCase(cfg config.Config, opts CLILLMOptions, logLevelOverride string) (usecase.ReviewUseCase, error) {
	deps, err := BuildCommonDependencies(cfg, opts, logLevelOverride)
	if err != nil {
		return nil, err
	}

	reviewPublisher, _, err := buildReviewPublishers(cfg, opts, deps.Logger)
	if err != nil {
		return nil, err
	}

	reviewer, err := reviewercodingagent.NewReviewer(deps.TracedGenerator, reviewercodingagent.Config{
		Agent:    deps.CodingAgentConfig.Agent,
		Provider: deps.CodingAgentConfig.Provider,
		Model:    deps.CodingAgentConfig.Model,
	}, deps.Logger)
	if err != nil {
		return nil, err
	}

	reviewUseCase, err := usecase.NewReviewUseCase(
		rulepack.NewCoreRulePackProvider(),
		reviewer,
		reviewPublisher,
		deps.Logger,
	)
	if err != nil {
		return nil, err
	}

	return reviewUseCase, nil
}

// BuildOverviewUseCase builds an overview usecase.
func BuildOverviewUseCase(cfg config.Config, opts CLILLMOptions, logLevelOverride string) (usecase.OverviewUseCase, error) {
	deps, err := BuildCommonDependencies(cfg, opts, logLevelOverride)
	if err != nil {
		return nil, err
	}

	_, overviewPublisher, err := buildReviewPublishers(cfg, opts, deps.Logger)
	if err != nil {
		return nil, err
	}

	overviewGenerator, err := overviewcodingagent.NewOverviewGenerator(deps.TracedGenerator, overviewcodingagent.Config{
		Agent:    deps.CodingAgentConfig.Agent,
		Provider: deps.CodingAgentConfig.Provider,
		Model:    deps.CodingAgentConfig.Model,
	}, deps.Logger)
	if err != nil {
		return nil, err
	}

	issueAlignmentGenerator, err := issuealignment.NewIssueAlignmentGenerator(deps.TracedGenerator, issuealignment.Config{
		Agent:    deps.CodingAgentConfig.Agent,
		Provider: deps.CodingAgentConfig.Provider,
		Model:    deps.CodingAgentConfig.Model,
	}, deps.Logger)
	if err != nil {
		return nil, err
	}

	return usecase.NewOverviewUseCase(
		overviewGenerator,
		issueAlignmentGenerator,
		overviewPublisher,
		deps.Logger,
	)
}

// BuildAutogenUseCase builds an autogen usecase.
func BuildAutogenUseCase(cfg config.Config, opts CLILLMOptions, logLevelOverride string) (usecase.AutogenUseCase, error) {
	deps, err := BuildCommonDependencies(cfg, opts, logLevelOverride)
	if err != nil {
		return nil, err
	}

	publisher, err := buildAutogenPublisher(cfg, opts, deps.Logger)
	if err != nil {
		return nil, err
	}

	generator, err := autogencodingagent.NewGenerator(autogencodingagent.Config{
		Agent:    deps.CodingAgentConfig.Agent,
		Provider: deps.CodingAgentConfig.Provider,
		Model:    deps.CodingAgentConfig.Model,
	}, deps.Logger)
	if err != nil {
		return nil, err
	}

	return usecase.NewAutogenUseCase(
		generator,
		publisher,
		deps.Logger,
	)
}

// BuildReplyCommentUseCase builds a reply comment usecase.
func BuildReplyCommentUseCase(cfg config.Config, opts CLILLMOptions, logLevelOverride string) (usecase.ReplyCommentUseCase, error) {
	deps, err := BuildCommonDependencies(cfg, opts, logLevelOverride)
	if err != nil {
		return nil, err
	}

	publisher, err := buildReplyCommentPublisher(cfg, opts, deps.Logger)
	if err != nil {
		return nil, err
	}

	answerer, err := replycommentcodingagent.NewAnswerer(replycommentcodingagent.Config{
		Agent:    deps.CodingAgentConfig.Agent,
		Provider: deps.CodingAgentConfig.Provider,
		Model:    deps.CodingAgentConfig.Model,
	}, deps.Logger)
	if err != nil {
		return nil, err
	}

	return usecase.NewReplyCommentUseCase(
		deps.ReadOnlySanitizer,
		answerer,
		publisher,
		deps.Logger,
	)
}

// CommonDependencies captures shared build-time dependencies.
type CommonDependencies struct {
	Logger                 usecase.Logger
	TracedGenerator        contracts.LLMGenerator
	ReadOnlySanitizer      usecase.SafetySanitizer
	ReadWriteSanitizer     usecase.SafetySanitizer
	RecipeLoader           usecase.CustomRecipeLoader
	CodeEnvironmentFactory contracts.CodeEnvironmentFactory
	CodingAgentConfig      config.CodingAgentConfig
}

// BuildCommonDependencies builds shared dependencies used by multiple usecases.
func BuildCommonDependencies(cfg config.Config, opts CLILLMOptions, logLevelOverride string) (CommonDependencies, error) {
	logger, err := BuildLogger(cfg, logLevelOverride)
	if err != nil {
		return CommonDependencies{}, err
	}

	codeEnvironmentFactory := codeenvhost.NewFactory(codeenvhost.FactoryConfig{Logger: logger})

	llmSelection, err := ResolveLLMSelection(cfg, opts)
	if err != nil {
		return CommonDependencies{}, err
	}

	codingAgentConfig := ResolveCLICodingAgentConfig(cfg, opts)
	cfgWithOverrides := cfg
	cfgWithOverrides.CodingAgent = codingAgentConfig

	var formatter contracts.LLMGenerator
	if llmSelection.UseOpenAI {
		formatter = buildOpenAIGenerator(llmSelection)
	} else {
		formatter, err = buildCodingAgentGenerator(cfgWithOverrides, logger)
		if err != nil {
			return CommonDependencies{}, err
		}
	}

	tracedGenerator := llmtracing.NewGenerator(formatter, logger)
	readOnlySanitizer, err := safetysanitizer.NewSanitizer(tracedGenerator, safetysanitizer.Options{
		EnforceReadOnly: true,
	})
	if err != nil {
		return CommonDependencies{}, err
	}
	readWriteSanitizer, err := safetysanitizer.NewSanitizer(tracedGenerator, safetysanitizer.Options{
		EnforceReadOnly: false,
	})
	if err != nil {
		return CommonDependencies{}, err
	}
	recipeLoader, err := customrecipe.NewLoader(readOnlySanitizer, readWriteSanitizer, logger)
	if err != nil {
		return CommonDependencies{}, err
	}

	return CommonDependencies{
		Logger:                 logger,
		TracedGenerator:        tracedGenerator,
		ReadOnlySanitizer:      readOnlySanitizer,
		ReadWriteSanitizer:     readWriteSanitizer,
		RecipeLoader:           recipeLoader,
		CodeEnvironmentFactory: codeEnvironmentFactory,
		CodingAgentConfig:      codingAgentConfig,
	}, nil
}

func buildReviewPublishers(cfg config.Config, opts CLILLMOptions, logger usecase.Logger) (usecase.ReviewResultPublisher, usecase.OverviewPublisher, error) {
	if shouldUseCLIPublishers(opts, cfg) {
		provider := normalizeVCSProvider(opts.VCSProvider)
		switch provider {
		case "gitlab":
			gitlabClient := gitlabvcs.NewCLIClientWithConfig(gitlabvcs.CLIClientConfig{Host: opts.VCSHost})
			return routerpublisher.NewReviewPublisher(
					clipublisher.NewPublisher(os.Stdout),
					gitlabpublisher.NewPublisher(gitlabClient, logger),
				), routerpublisher.NewOverviewPublisher(
					clipublisher.NewOverviewPublisher(os.Stdout),
					gitlabpublisher.NewOverviewPublisher(gitlabClient, logger),
				), nil
		default:
			githubClient := githubvcs.NewCLIClient()
			return routerpublisher.NewReviewPublisher(
					clipublisher.NewPublisher(os.Stdout),
					githubpublisher.NewPublisher(githubClient, logger),
				), routerpublisher.NewOverviewPublisher(
					clipublisher.NewOverviewPublisher(os.Stdout),
					githubpublisher.NewOverviewPublisher(githubClient, logger),
				), nil
		}
	}

	client, err := newAppGitHubClient(cfg)
	if err != nil {
		return nil, nil, err
	}
	return githubpublisher.NewPublisher(client, logger), githubpublisher.NewOverviewPublisher(client, logger), nil
}

func buildAutogenPublisher(cfg config.Config, opts CLILLMOptions, logger usecase.Logger) (usecase.AutogenPublisher, error) {
	if shouldUseCLIPublishers(opts, cfg) {
		provider := normalizeVCSProvider(opts.VCSProvider)
		switch provider {
		case "gitlab":
			gitlabClient := gitlabvcs.NewCLIClientWithConfig(gitlabvcs.CLIClientConfig{Host: opts.VCSHost})
			return routerpublisher.NewAutogenPublisher(
				clipublisher.NewAutogenPublisher(os.Stdout),
				gitlabpublisher.NewAutogenPublisher(gitlabClient, logger),
			), nil
		default:
			githubClient := githubvcs.NewCLIClient()
			return routerpublisher.NewAutogenPublisher(
				clipublisher.NewAutogenPublisher(os.Stdout),
				githubpublisher.NewAutogenPublisher(githubClient, logger),
			), nil
		}
	}

	client, err := newAppGitHubClient(cfg)
	if err != nil {
		return nil, err
	}
	return githubpublisher.NewAutogenPublisher(client, logger), nil
}

func buildReplyCommentPublisher(cfg config.Config, opts CLILLMOptions, logger usecase.Logger) (usecase.ReplyCommentPublisher, error) {
	if shouldUseCLIPublishers(opts, cfg) {
		provider := normalizeVCSProvider(opts.VCSProvider)
		switch provider {
		case "gitlab":
			gitlabClient := gitlabvcs.NewCLIClientWithConfig(gitlabvcs.CLIClientConfig{Host: opts.VCSHost})
			return routerpublisher.NewReplyCommentPublisher(
				clipublisher.NewReplyCommentPublisher(os.Stdout),
				gitlabpublisher.NewReplyCommentPublisher(gitlabClient, logger),
			), nil
		default:
			githubClient := githubvcs.NewCLIClient()
			return routerpublisher.NewReplyCommentPublisher(
				clipublisher.NewReplyCommentPublisher(os.Stdout),
				githubpublisher.NewReplyCommentPublisher(githubClient, logger),
			), nil
		}
	}

	client, err := newAppGitHubClient(cfg)
	if err != nil {
		return nil, err
	}
	return githubpublisher.NewReplyCommentPublisher(client, logger), nil
}

func shouldUseCLIPublishers(opts CLILLMOptions, cfg config.Config) bool {
	if opts.ForceCLIPublishers {
		return true
	}
	return !hasGitHubAppConfig(cfg)
}

func normalizeVCSProvider(provider string) string {
	provider = strings.TrimSpace(strings.ToLower(provider))
	if provider == "" {
		return "github"
	}
	return provider
}

func hasGitHubAppConfig(cfg config.Config) bool {
	return strings.TrimSpace(cfg.Server.GitHub.AppID) != "" &&
		strings.TrimSpace(cfg.Server.GitHub.AppPrivateKey) != "" &&
		strings.TrimSpace(cfg.Server.GitHub.WebhookSecret) != ""
}

func newAppGitHubClient(cfg config.Config) (*githubvcs.AppClient, error) {
	if !hasGitHubAppConfig(cfg) {
		return nil, fmt.Errorf("github app credentials are required")
	}

	httpClient := &http.Client{Timeout: 60 * time.Second}
	client, err := githubvcs.NewAppClient(httpClient, githubvcs.AppClientConfig{
		APIBaseURL: cfg.Server.GitHub.APIBaseURL,
		AppID:      cfg.Server.GitHub.AppID,
		PrivateKey: cfg.Server.GitHub.AppPrivateKey,
	})
	if err != nil {
		return nil, err
	}
	return client, nil
}
