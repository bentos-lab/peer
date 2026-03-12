package wiring

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	autogencodingagent "bentos-backend/adapter/outbound/autogen/codingagent"
	codeenvhost "bentos-backend/adapter/outbound/codeenv/host"
	customrecipe "bentos-backend/adapter/outbound/customrecipe"
	llmtracing "bentos-backend/adapter/outbound/llm/tracing"
	overviewcodingagent "bentos-backend/adapter/outbound/overview/codingagent"
	clipublisher "bentos-backend/adapter/outbound/publisher/cli"
	githubpublisher "bentos-backend/adapter/outbound/publisher/github"
	routerpublisher "bentos-backend/adapter/outbound/publisher/router"
	replycommentcodingagent "bentos-backend/adapter/outbound/replycomment/codingagent"
	reviewercodingagent "bentos-backend/adapter/outbound/reviewer/codingagent"
	safetysanitizer "bentos-backend/adapter/outbound/safetysanitizer"
	githubvcs "bentos-backend/adapter/outbound/vcs/github"
	"bentos-backend/config"
	"bentos-backend/domain"
	"bentos-backend/usecase"
	"bentos-backend/usecase/contracts"
	"bentos-backend/usecase/rulepack"
)

// BuildChangeRequestUseCase builds a change request usecase after initializing the repo in codeenv.
func BuildChangeRequestUseCase(cfg config.Config, opts CLILLMOptions, logLevelOverride string, repoURL string) (usecase.ChangeRequestUseCase, error) {
	deps, err := buildCommonDependencies(cfg, opts, logLevelOverride, repoURL)
	if err != nil {
		return nil, err
	}

	reviewPublisher, overviewPublisher, err := buildReviewPublishers(cfg, opts, deps.logger)
	if err != nil {
		return nil, err
	}

	reviewer, err := reviewercodingagent.NewReviewer(deps.tracedGenerator, reviewercodingagent.Config{
		Agent:    deps.codingAgentConfig.Agent,
		Provider: deps.codingAgentConfig.Provider,
		Model:    deps.codingAgentConfig.Model,
	}, deps.logger)
	if err != nil {
		return nil, err
	}

	overviewGenerator, err := overviewcodingagent.NewOverviewGenerator(deps.tracedGenerator, overviewcodingagent.Config{
		Agent:    deps.codingAgentConfig.Agent,
		Provider: deps.codingAgentConfig.Provider,
		Model:    deps.codingAgentConfig.Model,
	}, deps.logger)
	if err != nil {
		return nil, err
	}

	reviewUseCase, err := usecase.NewReviewUseCase(
		rulepack.NewCoreRulePackProvider(),
		reviewer,
		reviewPublisher,
		deps.codeEnvironmentFactory,
		deps.logger,
	)
	if err != nil {
		return nil, err
	}

	overviewUseCase, err := usecase.NewOverviewUseCase(
		overviewGenerator,
		overviewPublisher,
		deps.codeEnvironmentFactory,
		deps.logger,
	)
	if err != nil {
		return nil, err
	}

	return usecase.NewChangeRequestUseCase(
		reviewUseCase,
		overviewUseCase,
		deps.codeEnvironmentFactory,
		deps.recipeLoader,
		deps.logger,
	)
}

// BuildAutogenUseCase builds an autogen usecase after initializing the repo in codeenv.
func BuildAutogenUseCase(cfg config.Config, opts CLILLMOptions, logLevelOverride string, repoURL string) (usecase.AutogenUseCase, error) {
	deps, err := buildCommonDependencies(cfg, opts, logLevelOverride, repoURL)
	if err != nil {
		return nil, err
	}

	publisher, err := buildAutogenPublisher(cfg, opts, deps.logger)
	if err != nil {
		return nil, err
	}

	generator, err := autogencodingagent.NewGenerator(autogencodingagent.Config{
		Agent:    deps.codingAgentConfig.Agent,
		Provider: deps.codingAgentConfig.Provider,
		Model:    deps.codingAgentConfig.Model,
	}, deps.logger)
	if err != nil {
		return nil, err
	}

	return usecase.NewAutogenUseCase(
		generator,
		publisher,
		deps.codeEnvironmentFactory,
		deps.recipeLoader,
		deps.logger,
	)
}

// BuildReplyCommentUseCase builds a reply comment usecase after initializing the repo in codeenv.
func BuildReplyCommentUseCase(cfg config.Config, opts CLILLMOptions, logLevelOverride string, repoURL string) (usecase.ReplyCommentUseCase, error) {
	deps, err := buildCommonDependencies(cfg, opts, logLevelOverride, repoURL)
	if err != nil {
		return nil, err
	}

	publisher, err := buildReplyCommentPublisher(cfg, opts, deps.logger)
	if err != nil {
		return nil, err
	}

	answerer, err := replycommentcodingagent.NewAnswerer(replycommentcodingagent.Config{
		Agent:    deps.codingAgentConfig.Agent,
		Provider: deps.codingAgentConfig.Provider,
		Model:    deps.codingAgentConfig.Model,
	}, deps.logger)
	if err != nil {
		return nil, err
	}

	return usecase.NewReplyCommentUseCase(
		deps.readOnlySanitizer,
		answerer,
		publisher,
		deps.codeEnvironmentFactory,
		deps.recipeLoader,
		deps.logger,
	)
}

type commonDependencies struct {
	logger                 usecase.Logger
	tracedGenerator        contracts.LLMGenerator
	readOnlySanitizer      usecase.SafetySanitizer
	readWriteSanitizer     usecase.SafetySanitizer
	recipeLoader           usecase.CustomRecipeLoader
	codeEnvironmentFactory contracts.CodeEnvironmentFactory
	codingAgentConfig      config.CodingAgentConfig
}

func buildCommonDependencies(cfg config.Config, opts CLILLMOptions, logLevelOverride string, repoURL string) (commonDependencies, error) {
	logger, err := BuildLogger(cfg, logLevelOverride)
	if err != nil {
		return commonDependencies{}, err
	}

	codeEnvironmentFactory := codeenvhost.NewFactory(codeenvhost.FactoryConfig{Logger: logger})
	if err := preInitRepo(codeEnvironmentFactory, repoURL); err != nil {
		return commonDependencies{}, err
	}

	llmSelection, err := ResolveLLMSelection(cfg, opts)
	if err != nil {
		return commonDependencies{}, err
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
			return commonDependencies{}, err
		}
	}

	tracedGenerator := llmtracing.NewGenerator(formatter, logger)
	readOnlySanitizer, err := safetysanitizer.NewSanitizer(tracedGenerator, safetysanitizer.Options{
		EnforceReadOnly: true,
	})
	if err != nil {
		return commonDependencies{}, err
	}
	readWriteSanitizer, err := safetysanitizer.NewSanitizer(tracedGenerator, safetysanitizer.Options{
		EnforceReadOnly: false,
	})
	if err != nil {
		return commonDependencies{}, err
	}
	recipeLoader, err := customrecipe.NewLoader(readOnlySanitizer, readWriteSanitizer, logger)
	if err != nil {
		return commonDependencies{}, err
	}

	return commonDependencies{
		logger:                 logger,
		tracedGenerator:        tracedGenerator,
		readOnlySanitizer:      readOnlySanitizer,
		readWriteSanitizer:     readWriteSanitizer,
		recipeLoader:           recipeLoader,
		codeEnvironmentFactory: codeEnvironmentFactory,
		codingAgentConfig:      codingAgentConfig,
	}, nil
}

func preInitRepo(factory contracts.CodeEnvironmentFactory, repoURL string) error {
	environment, err := factory.New(context.Background(), domain.CodeEnvironmentInitOptions{RepoURL: repoURL})
	if err != nil {
		return err
	}
	if err := environment.Cleanup(context.Background()); err != nil {
		return err
	}
	return nil
}

func buildReviewPublishers(cfg config.Config, opts CLILLMOptions, logger usecase.Logger) (usecase.ReviewResultPublisher, usecase.OverviewPublisher, error) {
	if shouldUseCLIPublishers(opts, cfg) {
		githubClient := githubvcs.NewCLIClient()
		return routerpublisher.NewReviewPublisher(
				clipublisher.NewPublisher(os.Stdout),
				githubpublisher.NewPublisher(githubClient, logger),
			), routerpublisher.NewOverviewPublisher(
				clipublisher.NewOverviewPublisher(os.Stdout),
				githubpublisher.NewOverviewPublisher(githubClient, logger),
			), nil
	}

	client, err := newAppGitHubClient(cfg)
	if err != nil {
		return nil, nil, err
	}
	return githubpublisher.NewPublisher(client, logger), githubpublisher.NewOverviewPublisher(client, logger), nil
}

func buildAutogenPublisher(cfg config.Config, opts CLILLMOptions, logger usecase.Logger) (usecase.AutogenPublisher, error) {
	if shouldUseCLIPublishers(opts, cfg) {
		githubClient := githubvcs.NewCLIClient()
		return routerpublisher.NewAutogenPublisher(
			clipublisher.NewAutogenPublisher(os.Stdout),
			githubpublisher.NewAutogenPublisher(githubClient, logger),
		), nil
	}

	client, err := newAppGitHubClient(cfg)
	if err != nil {
		return nil, err
	}
	return githubpublisher.NewAutogenPublisher(client, logger), nil
}

func buildReplyCommentPublisher(cfg config.Config, opts CLILLMOptions, logger usecase.Logger) (usecase.ReplyCommentPublisher, error) {
	if shouldUseCLIPublishers(opts, cfg) {
		githubClient := githubvcs.NewCLIClient()
		return routerpublisher.NewReplyCommentPublisher(
			clipublisher.NewReplyCommentPublisher(os.Stdout),
			githubpublisher.NewReplyCommentPublisher(githubClient, logger),
		), nil
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
