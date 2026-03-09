package wiring

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	cliinbound "bentos-backend/adapter/inbound/cli"
	codeenvhost "bentos-backend/adapter/outbound/codeenv/host"
	openai "bentos-backend/adapter/outbound/llm/openai"
	llmtracing "bentos-backend/adapter/outbound/llm/tracing"
	overviewcodingagent "bentos-backend/adapter/outbound/overview/codingagent"
	clipublisher "bentos-backend/adapter/outbound/publisher/cli"
	githubpublisher "bentos-backend/adapter/outbound/publisher/github"
	routerpublisher "bentos-backend/adapter/outbound/publisher/router"
	reviewercodingagent "bentos-backend/adapter/outbound/reviewer/codingagent"
	githubvcs "bentos-backend/adapter/outbound/vcs/github"
	"bentos-backend/config"
	"bentos-backend/usecase"
	"bentos-backend/usecase/rulepack"
)

// CLILLMOptions contains CLI-only LLM overrides.
type CLILLMOptions struct {
	OpenAIBaseURL string
	OpenAIModel   string
	OpenAIAPIKey  string
}

// BuildCLICommand wires dependencies for a single CLI review mode.
func BuildCLICommand(cfg config.Config, opts CLILLMOptions, logLevelOverride string) (*cliinbound.Command, error) {
	llmConfig, err := resolveCLILLMConfig(cfg, opts)
	if err != nil {
		return nil, err
	}
	logger, err := buildLogger(cfg, logLevelOverride)
	if err != nil {
		return nil, err
	}

	httpClient := &http.Client{Timeout: 600 * time.Second}
	llmClient := openai.NewClient(httpClient, llmConfig)
	tracedLLMClient := llmtracing.NewGenerator(llmClient, logger)
	codeEnvironmentFactory := codeenvhost.NewFactory(logger)
	codingAgentConfig := resolveServerCodingAgentConfig(cfg)
	codingReviewer, err := reviewercodingagent.NewReviewer(tracedLLMClient, reviewercodingagent.Config{
		Agent:    codingAgentConfig.Agent,
		Provider: codingAgentConfig.Provider,
		Model:    codingAgentConfig.Model,
	}, logger)
	if err != nil {
		return nil, err
	}
	codingOverview, err := overviewcodingagent.NewOverviewGenerator(tracedLLMClient, overviewcodingagent.Config{
		Agent:    codingAgentConfig.Agent,
		Provider: codingAgentConfig.Provider,
		Model:    codingAgentConfig.Model,
	}, logger)
	if err != nil {
		return nil, err
	}
	ruleProvider := rulepack.NewCoreRulePackProvider()
	githubClient := githubvcs.NewCLIClient()

	reviewPublisher := routerpublisher.NewReviewPublisher(
		clipublisher.NewPublisher(os.Stdout),
		githubpublisher.NewPublisher(githubClient, logger),
	)
	overviewPublisher := routerpublisher.NewOverviewPublisher(
		clipublisher.NewOverviewPublisher(os.Stdout),
		githubpublisher.NewOverviewPublisher(githubClient, logger),
	)

	reviewUseCase, err := usecase.NewReviewUseCase(
		ruleProvider,
		codingReviewer,
		reviewPublisher,
		codeEnvironmentFactory,
		logger,
	)
	if err != nil {
		return nil, err
	}
	overviewUseCase, err := usecase.NewOverviewUseCase(
		codingOverview,
		overviewPublisher,
		codeEnvironmentFactory,
		logger,
	)
	if err != nil {
		return nil, err
	}
	changeRequestUseCase, err := usecase.NewChangeRequestUseCase(
		reviewUseCase,
		overviewUseCase,
		logger,
	)
	if err != nil {
		return nil, err
	}

	return cliinbound.NewCommand(changeRequestUseCase, githubClient, logger), nil
}

func resolveCLILLMConfig(cfg config.Config, opts CLILLMOptions) (openai.ClientConfig, error) {
	effectiveConfig, err := ResolveEffectiveOpenAIConfig(cfg, opts)
	if err != nil {
		return openai.ClientConfig{}, err
	}

	resolvedAPIKey := strings.TrimSpace(cfg.OpenAI.APIKey)
	if strings.TrimSpace(opts.OpenAIAPIKey) != "" {
		resolvedAPIKey = strings.TrimSpace(opts.OpenAIAPIKey)
	}
	if resolvedAPIKey == "" {
		return openai.ClientConfig{}, fmt.Errorf("openai API key is required")
	}

	return openai.ClientConfig{
		BaseURL: effectiveConfig.BaseURL,
		APIKey:  resolvedAPIKey,
		Model:   effectiveConfig.Model,
	}, nil
}
