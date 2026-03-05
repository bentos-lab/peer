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
	clipublisher "bentos-backend/adapter/outbound/publisher/cli"
	githubpublisher "bentos-backend/adapter/outbound/publisher/github"
	reviewerllm "bentos-backend/adapter/outbound/reviewer/llm"
	githubvcs "bentos-backend/adapter/outbound/vcs/github"
	"bentos-backend/config"
	"bentos-backend/domain"
	sharedllm "bentos-backend/shared/llm"
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
func BuildCLICommand(cfg config.Config, opts CLILLMOptions, provider domain.ReviewInputProvider, publishType domain.ReviewPublishType, logLevelOverride string) (*cliinbound.Command, error) {
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
	llmReviewer, err := reviewerllm.NewReviewer(llmClient, logger)
	if err != nil {
		return nil, err
	}
	ruleProvider := rulepack.NewCoreRulePackProvider()

	inputProvider, providerName, providerClient, err := buildInputProvider(provider)
	if err != nil {
		return nil, err
	}
	publisher, err := buildPublisher(publishType, provider, providerClient, logger)
	if err != nil {
		return nil, err
	}

	useCase, err := usecase.NewReviewerUseCase(
		inputProvider,
		ruleProvider,
		llmReviewer,
		publisher,
		logger,
	)
	if err != nil {
		return nil, err
	}

	return buildCLIInboundCommand(providerName, useCase, logger)
}

func buildInputProvider(provider domain.ReviewInputProvider) (usecase.ReviewInputProvider, domain.ReviewInputProvider, any, error) {
	switch provider {
	case domain.ReviewInputProviderLocal:
		return cliinput.NewProvider(cliinput.NewGitChangeDetector()), domain.ReviewInputProviderLocal, nil, nil
	case domain.ReviewInputProviderGitHub:
		providerClient := githubvcs.NewClient()
		return githubinput.NewProvider(providerClient), domain.ReviewInputProviderGitHub, providerClient, nil
	default:
		return nil, "", nil, fmt.Errorf("unsupported review input provider: %s", provider)
	}
}

func buildPublisher(publishType domain.ReviewPublishType, provider domain.ReviewInputProvider, providerClient any, logger usecase.Logger) (usecase.ReviewResultPublisher, error) {
	switch publishType {
	case domain.ReviewPublishTypePrint:
		return clipublisher.NewPublisher(os.Stdout), nil
	case domain.ReviewPublishTypeComment:
		switch provider {
		case domain.ReviewInputProviderGitHub:
			githubClient, ok := providerClient.(*githubvcs.Client)
			if !ok || githubClient == nil {
				return nil, fmt.Errorf("provider client is not configured for provider: %s", provider)
			}
			return githubpublisher.NewPublisher(githubClient, logger), nil
		default:
			return nil, fmt.Errorf("publish type %q is not supported with provider %q", publishType, provider)
		}
	default:
		return nil, fmt.Errorf("unsupported review publish type: %s", publishType)
	}
}

func buildCLIInboundCommand(providerName domain.ReviewInputProvider, reviewUseCase usecase.ReviewUseCase, logger usecase.Logger) (*cliinbound.Command, error) {
	switch providerName {
	case domain.ReviewInputProviderLocal:
		return cliinbound.NewLocalCommand(reviewUseCase, logger), nil
	case domain.ReviewInputProviderGitHub:
		return cliinbound.NewGitHubPRCommand(reviewUseCase, logger), nil
	default:
		return nil, fmt.Errorf("unsupported review input provider: %s", providerName)
	}
}

func validateCLISelection(provider domain.ReviewInputProvider, publishType domain.ReviewPublishType) error {
	allowedPublishTypesByProvider := map[domain.ReviewInputProvider]map[domain.ReviewPublishType]struct{}{
		domain.ReviewInputProviderLocal: {
			domain.ReviewPublishTypePrint: {},
		},
		domain.ReviewInputProviderGitHub: {
			domain.ReviewPublishTypePrint:   {},
			domain.ReviewPublishTypeComment: {},
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
	baseURLInput := strings.TrimSpace(cfg.OpenAIBaseURL)
	if strings.TrimSpace(opts.OpenAIBaseURL) != "" {
		baseURLInput = strings.TrimSpace(opts.OpenAIBaseURL)
	}
	resolvedBaseURL, resolvedModel, _, err := sharedllm.ResolveBaseURLAndModel(baseURLInput, cfg.OpenAIModel, opts.OpenAIModel)
	if err != nil {
		return openai.ClientConfig{}, err
	}

	resolvedAPIKey := strings.TrimSpace(cfg.OpenAIAPIKey)
	if strings.TrimSpace(opts.OpenAIAPIKey) != "" {
		resolvedAPIKey = strings.TrimSpace(opts.OpenAIAPIKey)
	}
	if resolvedAPIKey == "" {
		return openai.ClientConfig{}, fmt.Errorf("openai API key is required")
	}

	return openai.ClientConfig{
		BaseURL: resolvedBaseURL,
		APIKey:  resolvedAPIKey,
		Model:   resolvedModel,
	}, nil
}
