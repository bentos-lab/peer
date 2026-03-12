package main

import (
	"context"
	"net/http"
	"testing"

	cliinbound "bentos-backend/adapter/inbound/cli"
	githubvcs "bentos-backend/adapter/outbound/vcs/github"
	"bentos-backend/config"
	"bentos-backend/usecase"
	"bentos-backend/wiring"
	"github.com/stretchr/testify/require"
)

type mainTestChangeRequestUseCase struct {
	requests []usecase.ChangeRequestRequest
}

func (u *mainTestChangeRequestUseCase) Execute(_ context.Context, request usecase.ChangeRequestRequest) (usecase.ChangeRequestExecutionResult, error) {
	u.requests = append(u.requests, request)
	return usecase.ChangeRequestExecutionResult{}, nil
}

type mainTestGitHubClient struct{}

func (c *mainTestGitHubClient) ResolveRepository(_ context.Context, _ string) (string, error) {
	return "org/repo", nil
}

func (c *mainTestGitHubClient) GetPullRequestInfo(_ context.Context, _ string, _ int) (githubvcs.PullRequestInfo, error) {
	return githubvcs.PullRequestInfo{}, nil
}

func TestRunCLIResolvesSuggestFlagPrecedence(t *testing.T) {
	testCases := []struct {
		name            string
		args            []string
		envDefault      bool
		expectedSuggest bool
		explicit        bool
	}{
		{
			name:            "flag absent uses config false",
			args:            []string{},
			envDefault:      false,
			expectedSuggest: false,
			explicit:        false,
		},
		{
			name:            "flag absent uses config true",
			args:            []string{},
			envDefault:      true,
			expectedSuggest: true,
			explicit:        false,
		},
		{
			name:            "explicit false overrides config true",
			args:            []string{"--suggest=false"},
			envDefault:      true,
			expectedSuggest: false,
			explicit:        true,
		},
		{
			name:            "explicit true overrides config false",
			args:            []string{"--suggest"},
			envDefault:      false,
			expectedSuggest: true,
			explicit:        true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			changeRequestUseCase := &mainTestChangeRequestUseCase{}
			githubClient := &mainTestGitHubClient{}

			args := append([]string{"review"}, testCase.args...)

			deps := autogitDeps{
				loadConfig: func() (config.Config, error) {
					return config.Config{
						LogLevel: "info",
						CodingAgent: config.CodingAgentConfig{
							Agent: "opencode",
						},
						SuggestedChanges: config.SuggestedChangesConfig{
							Enabled: testCase.envDefault,
						},
					}, nil
				},
				buildReviewCommand: func(_ config.Config, _ wiring.CLILLMOptions, _ string) (*cliinbound.ReviewCommand, error) {
					builder := func(_ string) (usecase.ChangeRequestUseCase, error) {
						return changeRequestUseCase, nil
					}
					return cliinbound.NewReviewCommand(builder, githubClient, nil), nil
				},
				buildOverviewCommand: func(_ config.Config, _ wiring.CLILLMOptions, _ string) (*cliinbound.OverviewCommand, error) {
					return nil, nil
				},
				buildAutogenCommand: func(_ config.Config, _ wiring.CLILLMOptions, _ string) (*cliinbound.AutogenCommand, error) {
					return nil, nil
				},
				buildReplyCommentCommand: func(_ config.Config, _ wiring.CLILLMOptions, _ string) (*cliinbound.ReplyCommentCommand, error) {
					return nil, nil
				},
				buildGitHubHandler: func(config.Config) (http.Handler, error) {
					return http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), nil
				},
				listenAndServe: func(string, http.Handler) error {
					return nil
				},
			}

			err := runAutogit(context.Background(), args, deps)
			require.NoError(t, err)
			require.Len(t, changeRequestUseCase.requests, 1)
			require.True(t, changeRequestUseCase.requests[0].EnableReview)
			require.Equal(t, testCase.expectedSuggest, changeRequestUseCase.requests[0].EnableSuggestions)
			require.True(t, changeRequestUseCase.requests[0].ReviewExplicit)
			require.Equal(t, testCase.explicit, changeRequestUseCase.requests[0].SuggestionsExplicit)
		})
	}
}

func TestRunCLIOverviewSubcommandForcesOverviewOnly(t *testing.T) {
	changeRequestUseCase := &mainTestChangeRequestUseCase{}
	githubClient := &mainTestGitHubClient{}

	deps := autogitDeps{
		loadConfig: func() (config.Config, error) {
			return config.Config{
				LogLevel: "info",
				CodingAgent: config.CodingAgentConfig{
					Agent: "opencode",
				},
			}, nil
		},
		buildReviewCommand: func(_ config.Config, _ wiring.CLILLMOptions, _ string) (*cliinbound.ReviewCommand, error) {
			builder := func(_ string) (usecase.ChangeRequestUseCase, error) {
				return changeRequestUseCase, nil
			}
			return cliinbound.NewReviewCommand(builder, githubClient, nil), nil
		},
		buildOverviewCommand: func(_ config.Config, _ wiring.CLILLMOptions, _ string) (*cliinbound.OverviewCommand, error) {
			builder := func(_ string) (usecase.ChangeRequestUseCase, error) {
				return changeRequestUseCase, nil
			}
			return cliinbound.NewOverviewCommand(builder, githubClient, nil), nil
		},
		buildAutogenCommand: func(_ config.Config, _ wiring.CLILLMOptions, _ string) (*cliinbound.AutogenCommand, error) {
			return nil, nil
		},
		buildReplyCommentCommand: func(_ config.Config, _ wiring.CLILLMOptions, _ string) (*cliinbound.ReplyCommentCommand, error) {
			return nil, nil
		},
		buildGitHubHandler: func(config.Config) (http.Handler, error) {
			return http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), nil
		},
		listenAndServe: func(string, http.Handler) error {
			return nil
		},
	}

	err := runAutogit(context.Background(), []string{"overview"}, deps)
	require.NoError(t, err)
	require.Len(t, changeRequestUseCase.requests, 1)
	require.False(t, changeRequestUseCase.requests[0].EnableReview)
	require.True(t, changeRequestUseCase.requests[0].EnableOverview)
	require.True(t, changeRequestUseCase.requests[0].ReviewExplicit)
	require.True(t, changeRequestUseCase.requests[0].OverviewExplicit)
}

func TestWebhookRequiresProvider(t *testing.T) {
	deps := autogitDeps{
		loadConfig: func() (config.Config, error) {
			return config.Config{}, nil
		},
		buildGitHubHandler: func(config.Config) (http.Handler, error) {
			return http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), nil
		},
		listenAndServe: func(string, http.Handler) error {
			return nil
		},
	}

	err := runAutogit(context.Background(), []string{"webhook"}, deps)
	require.Error(t, err)
}

func TestWebhookRejectsUnsupportedProvider(t *testing.T) {
	deps := autogitDeps{
		loadConfig: func() (config.Config, error) {
			return config.Config{}, nil
		},
		buildGitHubHandler: func(config.Config) (http.Handler, error) {
			return http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), nil
		},
		listenAndServe: func(string, http.Handler) error {
			return nil
		},
	}

	err := runAutogit(context.Background(), []string{"webhook", "--vcs-provider", "gitlab"}, deps)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported vcs provider")
}

func TestWebhookOverridesConfig(t *testing.T) {
	var captured config.Config
	deps := autogitDeps{
		loadConfig: func() (config.Config, error) {
			return config.Config{
				LogLevel: "info",
				CodingAgent: config.CodingAgentConfig{
					Agent: "opencode",
				},
				Server: config.ServerConfig{
					Port: "8080",
					GitHub: config.GitHubConfig{
						WebhookSecret:           "secret",
						AppID:                   "123",
						AppPrivateKey:           "key",
						APIBaseURL:              "https://api.github.com",
						ReplyCommentTriggerName: "autogitbot",
					},
				},
			}, nil
		},
		buildGitHubHandler: func(cfg config.Config) (http.Handler, error) {
			captured = cfg
			return http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), nil
		},
		listenAndServe: func(string, http.Handler) error {
			return nil
		},
	}

	args := []string{
		"webhook",
		"--vcs-provider", "github",
		"--github-app-id", "999",
		"--overview-enabled=false",
		"--review-suggested-changes-max-workers", "9",
	}
	err := runAutogit(context.Background(), args, deps)
	require.NoError(t, err)
	require.Equal(t, "999", captured.Server.GitHub.AppID)
	require.NotNil(t, captured.OverviewEnabled)
	require.False(t, *captured.OverviewEnabled)
	require.Equal(t, 9, captured.SuggestedChanges.MaxWorkers)
}
