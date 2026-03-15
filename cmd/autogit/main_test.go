package main

import (
	"context"
	"net/http"
	"testing"

	cliinbound "bentos-backend/adapter/inbound/cli"
	githubvcs "bentos-backend/adapter/outbound/vcs/github"
	"bentos-backend/config"
	"bentos-backend/domain"
	"bentos-backend/usecase"
	uccontracts "bentos-backend/usecase/contracts"
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
	return githubvcs.PullRequestInfo{
		Repository:  "org/repo",
		Number:      7,
		Title:       "Title",
		Description: "Fixes #12",
		BaseRef:     "main",
		HeadRef:     "feature",
	}, nil
}

func (c *mainTestGitHubClient) GetIssue(_ context.Context, repository string, issueNumber int) (githubvcs.Issue, error) {
	return githubvcs.Issue{
		Repository: repository,
		Number:     issueNumber,
		Title:      "Issue",
		Body:       "Body",
	}, nil
}

func (c *mainTestGitHubClient) ListIssueComments(_ context.Context, _ string, _ int) ([]githubvcs.IssueComment, error) {
	return nil, nil
}

func boolPointer(value bool) *bool {
	return &value
}

type mainTestCodeEnvironment struct{}

func (e *mainTestCodeEnvironment) SetupAgent(_ context.Context, _ domain.CodingAgentSetupOptions) (uccontracts.CodingAgent, error) {
	return nil, nil
}

func (e *mainTestCodeEnvironment) LoadChangedFiles(_ context.Context, _ domain.CodeEnvironmentLoadOptions) ([]domain.ChangedFile, error) {
	return nil, nil
}

func (e *mainTestCodeEnvironment) ReadFile(_ context.Context, _ string, _ string) (string, bool, error) {
	return "", false, nil
}

func (e *mainTestCodeEnvironment) PushChanges(_ context.Context, _ domain.CodeEnvironmentPushOptions) (domain.CodeEnvironmentPushResult, error) {
	return domain.CodeEnvironmentPushResult{}, nil
}

func (e *mainTestCodeEnvironment) Cleanup(_ context.Context) error {
	return nil
}

type mainTestCodeEnvironmentFactory struct {
	environment uccontracts.CodeEnvironment
}

func (f *mainTestCodeEnvironmentFactory) New(_ context.Context, _ domain.CodeEnvironmentInitOptions) (uccontracts.CodeEnvironment, error) {
	if f.environment == nil {
		f.environment = &mainTestCodeEnvironment{}
	}
	return f.environment, nil
}

type mainTestRecipeLoader struct {
	recipe domain.CustomRecipe
}

func (l *mainTestRecipeLoader) Load(_ context.Context, _ uccontracts.CodeEnvironment, _ string) (domain.CustomRecipe, error) {
	return l.recipe, nil
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
			envFactory := &mainTestCodeEnvironmentFactory{}
			recipeLoader := &mainTestRecipeLoader{
				recipe: domain.CustomRecipe{
					ReviewSuggestions: boolPointer(testCase.envDefault),
				},
			}

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
					return cliinbound.NewReviewCommand(builder, githubClient, envFactory, recipeLoader, nil), nil
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
			return cliinbound.NewReviewCommand(builder, githubClient, &mainTestCodeEnvironmentFactory{}, &mainTestRecipeLoader{}, nil), nil
		},
		buildOverviewCommand: func(_ config.Config, _ wiring.CLILLMOptions, _ string) (*cliinbound.OverviewCommand, error) {
			builder := func(_ string) (usecase.ChangeRequestUseCase, error) {
				return changeRequestUseCase, nil
			}
			return cliinbound.NewOverviewCommand(builder, githubClient, &mainTestCodeEnvironmentFactory{}, &mainTestRecipeLoader{}, nil), nil
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
}

func TestRunCLIOverviewIssueAlignmentFlag(t *testing.T) {
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
		buildOverviewCommand: func(_ config.Config, _ wiring.CLILLMOptions, _ string) (*cliinbound.OverviewCommand, error) {
			builder := func(_ string) (usecase.ChangeRequestUseCase, error) {
				return changeRequestUseCase, nil
			}
			return cliinbound.NewOverviewCommand(builder, githubClient, &mainTestCodeEnvironmentFactory{}, &mainTestRecipeLoader{}, nil), nil
		},
		buildGitHubHandler: func(config.Config) (http.Handler, error) {
			return http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), nil
		},
		listenAndServe: func(string, http.Handler) error {
			return nil
		},
	}

	err := runAutogit(context.Background(), []string{"overview", "--issue-alignment", "--change-request", "7"}, deps)
	require.NoError(t, err)
	require.Len(t, changeRequestUseCase.requests, 1)
	require.NotEmpty(t, changeRequestUseCase.requests[0].OverviewIssueAlignment.Candidates)
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
