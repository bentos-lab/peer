package wiring

import (
	"net/http"
	"time"

	githubinbound "bentos-backend/adapter/inbound/http/github"
	gitlabinbound "bentos-backend/adapter/inbound/http/gitlab"
	githubinput "bentos-backend/adapter/outbound/input/github"
	gitlabinput "bentos-backend/adapter/outbound/input/gitlab"
	openai "bentos-backend/adapter/outbound/llm/openai"
	githubpublisher "bentos-backend/adapter/outbound/publisher/github"
	gitlabpublisher "bentos-backend/adapter/outbound/publisher/gitlab"
	reviewerllm "bentos-backend/adapter/outbound/reviewer/llm"
	githubvcs "bentos-backend/adapter/outbound/vcs/github"
	gitlabvcs "bentos-backend/adapter/outbound/vcs/gitlab"
	"bentos-backend/config"
	"bentos-backend/usecase"
	"bentos-backend/usecase/rulepack"
)

const serverLLMTimeout = 600 * time.Second

// BuildGitHubHandler wires dependencies for GitHub webhook flow.
func BuildGitHubHandler(cfg config.Config) (*githubinbound.Handler, error) {
	logger, err := buildLogger(cfg, "")
	if err != nil {
		return nil, err
	}
	llmReviewer, err := buildServerLLMReviewer(cfg, logger)
	if err != nil {
		return nil, err
	}
	ghClient := githubvcs.NewClient()
	uc, err := usecase.NewReviewerUseCase(
		githubinput.NewProvider(ghClient),
		rulepack.NewCoreRulePackProvider(),
		llmReviewer,
		githubpublisher.NewPublisher(ghClient, logger),
		logger,
	)
	if err != nil {
		return nil, err
	}
	return githubinbound.NewHandler(uc, logger), nil
}

// BuildGitLabHandler wires dependencies for GitLab webhook flow.
func BuildGitLabHandler(cfg config.Config) (*gitlabinbound.Handler, error) {
	logger, err := buildLogger(cfg, "")
	if err != nil {
		return nil, err
	}
	llmReviewer, err := buildServerLLMReviewer(cfg, logger)
	if err != nil {
		return nil, err
	}
	glClient := gitlabvcs.NewClient()
	uc, err := usecase.NewReviewerUseCase(
		gitlabinput.NewProvider(glClient),
		rulepack.NewCoreRulePackProvider(),
		llmReviewer,
		gitlabpublisher.NewPublisher(glClient, logger),
		logger,
	)
	if err != nil {
		return nil, err
	}
	return gitlabinbound.NewHandler(uc, logger), nil
}

func buildServerLLMReviewer(cfg config.Config, logger usecase.Logger) (*reviewerllm.Reviewer, error) {
	httpClient := &http.Client{Timeout: serverLLMTimeout}
	llmClient := openai.NewClient(httpClient, buildOpenAIClientConfig(cfg))
	return reviewerllm.NewReviewer(llmClient, logger)
}

func buildOpenAIClientConfig(cfg config.Config) openai.ClientConfig {
	return openai.ClientConfig{
		BaseURL: cfg.OpenAIBaseURL,
		APIKey:  cfg.OpenAIAPIKey,
		Model:   cfg.OpenAIModel,
	}
}
