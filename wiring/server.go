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

// BuildGitHubHandler wires dependencies for GitHub webhook flow.
func BuildGitHubHandler(cfg config.Config) (*githubinbound.Handler, error) {
	httpClient := &http.Client{Timeout: 60 * time.Second}
	llmClient := openai.NewClient(httpClient, openai.ClientConfig{
		BaseURL: cfg.OpenAIBaseURL,
		APIKey:  cfg.OpenAIAPIKey,
		Model:   cfg.OpenAIModel,
		Timeout: 60 * time.Second,
	})
	llmReviewer, err := reviewerllm.NewReviewer(llmClient)
	if err != nil {
		return nil, err
	}
	ghClient := githubvcs.NewClient()
	uc, err := usecase.NewReviewerUseCase(
		githubinput.NewProvider(ghClient),
		rulepack.NewCoreRulePackProvider(),
		llmReviewer,
		githubpublisher.NewPublisher(ghClient),
	)
	if err != nil {
		return nil, err
	}
	return githubinbound.NewHandler(uc), nil
}

// BuildGitLabHandler wires dependencies for GitLab webhook flow.
func BuildGitLabHandler(cfg config.Config) (*gitlabinbound.Handler, error) {
	httpClient := &http.Client{Timeout: 60 * time.Second}
	llmClient := openai.NewClient(httpClient, openai.ClientConfig{
		BaseURL: cfg.OpenAIBaseURL,
		APIKey:  cfg.OpenAIAPIKey,
		Model:   cfg.OpenAIModel,
		Timeout: 60 * time.Second,
	})
	llmReviewer, err := reviewerllm.NewReviewer(llmClient)
	if err != nil {
		return nil, err
	}
	glClient := gitlabvcs.NewClient()
	uc, err := usecase.NewReviewerUseCase(
		gitlabinput.NewProvider(glClient),
		rulepack.NewCoreRulePackProvider(),
		llmReviewer,
		gitlabpublisher.NewPublisher(glClient),
	)
	if err != nil {
		return nil, err
	}
	return gitlabinbound.NewHandler(uc), nil
}
