package wiring

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	cliinbound "bentos-backend/adapter/inbound/cli"
	cliinput "bentos-backend/adapter/outbound/input/cli"
	openai "bentos-backend/adapter/outbound/llm/openai"
	clipublisher "bentos-backend/adapter/outbound/publisher/cli"
	reviewerllm "bentos-backend/adapter/outbound/reviewer/llm"
	"bentos-backend/config"
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

// BuildCLICommand wires dependencies for CLI review.
func BuildCLICommand(cfg config.Config, writer io.Writer, opts CLILLMOptions) (*cliinbound.Command, error) {
	llmConfig, err := resolveCLILLMConfig(cfg, opts)
	if err != nil {
		return nil, err
	}

	httpClient := &http.Client{Timeout: 60 * time.Second}
	llmClient := openai.NewClient(httpClient, llmConfig)
	llmReviewer, err := reviewerllm.NewReviewer(llmClient)
	if err != nil {
		return nil, err
	}
	uc, err := usecase.NewReviewerUseCase(
		cliinput.NewProvider(cliinput.NewGitChangeDetector()),
		rulepack.NewCoreRulePackProvider(),
		llmReviewer,
		clipublisher.NewPublisher(writer),
	)
	if err != nil {
		return nil, err
	}
	return cliinbound.NewCommand(uc), nil
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
		Timeout: 60 * time.Second,
	}, nil
}
