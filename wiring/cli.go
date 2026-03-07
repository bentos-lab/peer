package wiring

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	cliinbound "bentos-backend/adapter/inbound/cli"
	cliinput "bentos-backend/adapter/outbound/input/cli"
	githubinput "bentos-backend/adapter/outbound/input/github"
	openai "bentos-backend/adapter/outbound/llm/openai"
	llmtracing "bentos-backend/adapter/outbound/llm/tracing"
	overviewllm "bentos-backend/adapter/outbound/overview/llm"
	clipublisher "bentos-backend/adapter/outbound/publisher/cli"
	githubpublisher "bentos-backend/adapter/outbound/publisher/github"
	reviewerllm "bentos-backend/adapter/outbound/reviewer/llm"
	githubvcs "bentos-backend/adapter/outbound/vcs/github"
	"bentos-backend/config"
	"bentos-backend/domain"
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
func BuildCLICommand(cfg config.Config, opts CLILLMOptions, provider domain.ChangeRequestInputProvider, publishType domain.ChangeRequestPublishType, logLevelOverride string) (*cliinbound.Command, error) {
	if err := validateCLISelection(provider, publishType); err != nil {
		return nil, err
	}

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
	llmReviewer, err := reviewerllm.NewReviewer(tracedLLMClient, logger)
	if err != nil {
		return nil, err
	}
	llmOverview, err := overviewllm.NewOverviewGenerator(tracedLLMClient, logger)
	if err != nil {
		return nil, err
	}
	ruleProvider := rulepack.NewCoreRulePackProvider()

	inputProvider, providerName, providerClient, err := buildInputProvider(provider)
	if err != nil {
		return nil, err
	}
	reviewPublisher, overviewPublisher, err := buildPublishers(publishType, provider, providerClient, logger)
	if err != nil {
		return nil, err
	}
	reviewOptions, err := reviewUseCaseOptionsFromConfig(cfg, llmReviewer)
	if err != nil {
		return nil, err
	}

	reviewUseCase, err := usecase.NewReviewUseCase(
		ruleProvider,
		llmReviewer,
		reviewPublisher,
		logger,
		reviewOptions...,
	)
	if err != nil {
		return nil, err
	}
	overviewUseCase, err := usecase.NewOverviewUseCase(
		llmOverview,
		overviewPublisher,
		logger,
	)
	if err != nil {
		return nil, err
	}
	changeRequestUseCase, err := usecase.NewChangeRequestUseCase(
		inputProvider,
		reviewUseCase,
		overviewUseCase,
		logger,
	)
	if err != nil {
		return nil, err
	}

	return buildCLIInboundCommand(providerName, changeRequestUseCase, logger)
}

func buildInputProvider(provider domain.ChangeRequestInputProvider) (usecase.ChangeRequestInputProvider, domain.ChangeRequestInputProvider, any, error) {
	switch provider {
	case domain.ChangeRequestInputProviderLocal:
		return cliinput.NewProvider(cliinput.NewGitChangeDetector()), domain.ChangeRequestInputProviderLocal, nil, nil
	case domain.ChangeRequestInputProviderGitHub:
		providerClient := githubvcs.NewCLIClient()
		return githubinput.NewProvider(providerClient), domain.ChangeRequestInputProviderGitHub, providerClient, nil
	default:
		return nil, "", nil, fmt.Errorf("unsupported review input provider: %s", provider)
	}
}

func buildPublishers(publishType domain.ChangeRequestPublishType, provider domain.ChangeRequestInputProvider, providerClient any, logger usecase.Logger) (usecase.ReviewResultPublisher, usecase.OverviewPublisher, error) {
	switch publishType {
	case domain.ChangeRequestPublishTypePrint:
		return clipublisher.NewPublisher(os.Stdout), clipublisher.NewOverviewPublisher(os.Stdout), nil
	case domain.ChangeRequestPublishTypeComment:
		switch provider {
		case domain.ChangeRequestInputProviderGitHub:
			githubClient, ok := providerClient.(*githubvcs.CLIClient)
			if !ok || githubClient == nil {
				return nil, nil, fmt.Errorf("provider client is not configured for provider: %s", provider)
			}
			return githubpublisher.NewPublisher(githubClient, logger), githubpublisher.NewOverviewPublisher(githubClient, logger), nil
		default:
			return nil, nil, fmt.Errorf("publish type %q is not supported with provider %q", publishType, provider)
		}
	default:
		return nil, nil, fmt.Errorf("unsupported review publish type: %s", publishType)
	}
}

func buildCLIInboundCommand(providerName domain.ChangeRequestInputProvider, changeRequestUseCase usecase.ChangeRequestUseCase, logger usecase.Logger) (*cliinbound.Command, error) {
	switch providerName {
	case domain.ChangeRequestInputProviderLocal:
		return cliinbound.NewLocalCommand(changeRequestUseCase, logger), nil
	case domain.ChangeRequestInputProviderGitHub:
		return cliinbound.NewGitHubPRCommand(changeRequestUseCase, logger), nil
	default:
		return nil, fmt.Errorf("unsupported review input provider: %s", providerName)
	}
}

func validateCLISelection(provider domain.ChangeRequestInputProvider, publishType domain.ChangeRequestPublishType) error {
	allowedPublishTypesByProvider := map[domain.ChangeRequestInputProvider]map[domain.ChangeRequestPublishType]struct{}{
		domain.ChangeRequestInputProviderLocal: {
			domain.ChangeRequestPublishTypePrint: {},
		},
		domain.ChangeRequestInputProviderGitHub: {
			domain.ChangeRequestPublishTypePrint:   {},
			domain.ChangeRequestPublishTypeComment: {},
		},
	}

	allowedPublishTypes, ok := allowedPublishTypesByProvider[provider]
	if !ok {
		return fmt.Errorf("unsupported review input provider: %s", provider)
	}
	if _, ok := allowedPublishTypes[publishType]; !ok {
		return fmt.Errorf("publish type %q is not supported with provider %q", publishType, provider)
	}
	return nil
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
