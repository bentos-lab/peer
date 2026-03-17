package main

import (
	"context"
	"net/http"
	"testing"

	cliinbound "bentos-backend/adapter/inbound/cli"
	gitlabinbound "bentos-backend/adapter/inbound/http/gitlab"
	"bentos-backend/config"
	"bentos-backend/domain"
	"bentos-backend/usecase"
	uccontracts "bentos-backend/usecase/contracts"
	"bentos-backend/wiring"
	"github.com/stretchr/testify/require"
)

type mainTestReviewUseCase struct {
	requests []usecase.ReviewRequest
}

func (u *mainTestReviewUseCase) Execute(_ context.Context, request usecase.ReviewRequest) (usecase.ReviewExecutionResult, error) {
	u.requests = append(u.requests, request)
	return usecase.ReviewExecutionResult{}, nil
}

type mainTestOverviewUseCase struct {
	requests []usecase.OverviewRequest
}

func (u *mainTestOverviewUseCase) Execute(_ context.Context, request usecase.OverviewRequest) (usecase.OverviewExecutionResult, error) {
	u.requests = append(u.requests, request)
	return usecase.OverviewExecutionResult{}, nil
}

type mainTestGitHubClient struct{}

func (c *mainTestGitHubClient) ResolveRepository(_ context.Context, _ string) (string, error) {
	return "org/repo", nil
}

func (c *mainTestGitHubClient) GetPullRequestInfo(_ context.Context, _ string, _ int) (domain.ChangeRequestInfo, error) {
	return domain.ChangeRequestInfo{
		Repository:  "org/repo",
		Number:      7,
		Title:       "Title",
		Description: "Fixes #12",
		BaseRef:     "main",
		HeadRef:     "feature",
	}, nil
}

func (c *mainTestGitHubClient) GetIssue(_ context.Context, repository string, issueNumber int) (domain.Issue, error) {
	return domain.Issue{
		Repository: repository,
		Number:     issueNumber,
		Title:      "Issue",
		Body:       "Body",
	}, nil
}

func (c *mainTestGitHubClient) ListIssueComments(_ context.Context, _ string, _ int) ([]domain.IssueComment, error) {
	return nil, nil
}

func (c *mainTestGitHubClient) ListChangeRequestComments(_ context.Context, _ string, _ int) ([]domain.IssueComment, error) {
	return nil, nil
}

func (c *mainTestGitHubClient) GetPullRequestReview(_ context.Context, _ string, _ int, _ int64) (domain.ReviewSummary, error) {
	return domain.ReviewSummary{}, nil
}

func (c *mainTestGitHubClient) GetIssueComment(_ context.Context, _ string, _ int, _ int64) (domain.IssueComment, error) {
	return domain.IssueComment{}, nil
}

func (c *mainTestGitHubClient) GetReviewComment(_ context.Context, _ string, _ int, _ int64) (domain.ReviewComment, error) {
	return domain.ReviewComment{}, nil
}

func (c *mainTestGitHubClient) ListReviewComments(_ context.Context, _ string, _ int) ([]domain.ReviewComment, error) {
	return nil, nil
}

type mainTestGitLabClient = mainTestGitHubClient

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
			reviewUseCase := &mainTestReviewUseCase{}
			githubClient := &mainTestGitHubClient{}
			envFactory := &mainTestCodeEnvironmentFactory{}
			recipeLoader := &mainTestRecipeLoader{
				recipe: domain.CustomRecipe{
					ReviewSuggestions: boolPointer(testCase.envDefault),
				},
			}

			args := append([]string{"review"}, testCase.args...)
			args = append(args, "--vcs-provider", "github")

			deps := autogitDeps{
				loadConfig: func() (config.Config, error) {
					return config.Config{
						LogLevel: "info",
						CodingAgent: config.CodingAgentConfig{
							Agent: "opencode",
						},
						Review: config.ReviewConfig{
							SuggestedChangesEnabled: testCase.envDefault,
						},
					}, nil
				},
				buildReviewCommand: func(_ config.Config, _ wiring.CLILLMOptions, _ string) (*cliinbound.ReviewCommand, error) {
					builder := func(_ string) (usecase.ReviewUseCase, error) {
						return reviewUseCase, nil
					}
					resolver := cliinbound.StaticVCSClients{GitHub: githubClient}
					return cliinbound.NewReviewCommand(builder, resolver, envFactory, recipeLoader, nil), nil
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
				buildGitLabHandler: func(config.Config) (http.Handler, *gitlabinbound.HookSyncer, error) {
					return http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), nil, nil
				},
				listenAndServe: func(string, http.Handler) error {
					return nil
				},
			}

			err := runAutogit(context.Background(), args, deps)
			require.NoError(t, err)
			require.Len(t, reviewUseCase.requests, 1)
			require.Equal(t, testCase.expectedSuggest, reviewUseCase.requests[0].Suggestions)
		})
	}
}

func TestRunCLIOverviewSubcommandForcesOverviewOnly(t *testing.T) {
	overviewUseCase := &mainTestOverviewUseCase{}
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
			builder := func(_ string) (usecase.ReviewUseCase, error) {
				return &mainTestReviewUseCase{}, nil
			}
			resolver := cliinbound.StaticVCSClients{GitHub: githubClient}
			return cliinbound.NewReviewCommand(builder, resolver, &mainTestCodeEnvironmentFactory{}, &mainTestRecipeLoader{}, nil), nil
		},
		buildOverviewCommand: func(_ config.Config, _ wiring.CLILLMOptions, _ string) (*cliinbound.OverviewCommand, error) {
			builder := func(_ string) (usecase.OverviewUseCase, error) {
				return overviewUseCase, nil
			}
			resolver := cliinbound.StaticVCSClients{GitHub: githubClient}
			return cliinbound.NewOverviewCommand(builder, resolver, &mainTestCodeEnvironmentFactory{}, &mainTestRecipeLoader{}, nil), nil
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
		buildGitLabHandler: func(config.Config) (http.Handler, *gitlabinbound.HookSyncer, error) {
			return http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), nil, nil
		},
		listenAndServe: func(string, http.Handler) error {
			return nil
		},
	}

	err := runAutogit(context.Background(), []string{"overview", "--vcs-provider", "github"}, deps)
	require.NoError(t, err)
	require.Len(t, overviewUseCase.requests, 1)
}

func TestCLIAutoDetectVCSProviderFromRepo(t *testing.T) {
	reviewUseCase := &mainTestReviewUseCase{}
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
			builder := func(_ string) (usecase.ReviewUseCase, error) {
				return reviewUseCase, nil
			}
			resolver := cliinbound.StaticVCSClients{GitHub: githubClient}
			return cliinbound.NewReviewCommand(builder, resolver, &mainTestCodeEnvironmentFactory{}, &mainTestRecipeLoader{}, nil), nil
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
		buildGitLabHandler: func(config.Config) (http.Handler, *gitlabinbound.HookSyncer, error) {
			return http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), nil, nil
		},
		listenAndServe: func(string, http.Handler) error {
			return nil
		},
	}

	args := []string{"review", "--repo", "https://github.com/owner/repo.git"}
	err := runAutogit(context.Background(), args, deps)
	require.NoError(t, err)
	require.Len(t, reviewUseCase.requests, 1)
}

func TestCLIAutoDetectVCSProviderFromOriginURL(t *testing.T) {
	overviewUseCase := &mainTestOverviewUseCase{}
	gitlabClient := &mainTestGitLabClient{}

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
			return nil, nil
		},
		buildOverviewCommand: func(_ config.Config, _ wiring.CLILLMOptions, _ string) (*cliinbound.OverviewCommand, error) {
			builder := func(_ string) (usecase.OverviewUseCase, error) {
				return overviewUseCase, nil
			}
			resolver := cliinbound.StaticVCSClients{GitLab: gitlabClient}
			return cliinbound.NewOverviewCommand(builder, resolver, &mainTestCodeEnvironmentFactory{}, &mainTestRecipeLoader{}, nil), nil
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
		buildGitLabHandler: func(config.Config) (http.Handler, *gitlabinbound.HookSyncer, error) {
			return http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), nil, nil
		},
		listenAndServe: func(string, http.Handler) error {
			return nil
		},
		resolveOriginURL: func() (string, error) {
			return "https://gitlab.example.com/group/project.git", nil
		},
	}

	err := runAutogit(context.Background(), []string{"overview"}, deps)
	require.NoError(t, err)
	require.Len(t, overviewUseCase.requests, 1)
}

func TestRunCLIOverviewIssueAlignmentFlag(t *testing.T) {
	overviewUseCase := &mainTestOverviewUseCase{}
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
			builder := func(_ string) (usecase.OverviewUseCase, error) {
				return overviewUseCase, nil
			}
			resolver := cliinbound.StaticVCSClients{GitHub: githubClient}
			return cliinbound.NewOverviewCommand(builder, resolver, &mainTestCodeEnvironmentFactory{}, &mainTestRecipeLoader{}, nil), nil
		},
		buildGitHubHandler: func(config.Config) (http.Handler, error) {
			return http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), nil
		},
		buildGitLabHandler: func(config.Config) (http.Handler, *gitlabinbound.HookSyncer, error) {
			return http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), nil, nil
		},
		listenAndServe: func(string, http.Handler) error {
			return nil
		},
	}

	err := runAutogit(context.Background(), []string{"overview", "--issue-alignment", "--change-request", "7", "--vcs-provider", "github"}, deps)
	require.NoError(t, err)
	require.Len(t, overviewUseCase.requests, 1)
	require.NotEmpty(t, overviewUseCase.requests[0].IssueAlignment.Candidates)
}

func TestWebhookRequiresProvider(t *testing.T) {
	deps := autogitDeps{
		loadConfig: func() (config.Config, error) {
			return config.Config{}, nil
		},
		buildGitHubHandler: func(config.Config) (http.Handler, error) {
			return http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), nil
		},
		buildGitLabHandler: func(config.Config) (http.Handler, *gitlabinbound.HookSyncer, error) {
			return http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), nil, nil
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
		buildGitLabHandler: func(config.Config) (http.Handler, *gitlabinbound.HookSyncer, error) {
			return http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), nil, nil
		},
		listenAndServe: func(string, http.Handler) error {
			return nil
		},
	}

	err := runAutogit(context.Background(), []string{"webhook", "--vcs-provider", "bitbucket"}, deps)
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
						WebhookSecret: "secret",
						AppID:         "123",
						AppPrivateKey: "key",
						APIBaseURL:    "https://api.github.com",
					},
				},
				ReplyComment: config.ReplyCommentConfig{
					TriggerName: "autogitbot",
				},
			}, nil
		},
		buildGitHubHandler: func(cfg config.Config) (http.Handler, error) {
			captured = cfg
			return http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), nil
		},
		buildGitLabHandler: func(config.Config) (http.Handler, *gitlabinbound.HookSyncer, error) {
			return http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), nil, nil
		},
		listenAndServe: func(string, http.Handler) error {
			return nil
		},
	}

	args := []string{
		"webhook",
		"--vcs-provider", "github",
		"--github-app-id", "999",
		"--overview=false",
	}
	err := runAutogit(context.Background(), args, deps)
	require.NoError(t, err)
	require.Equal(t, "999", captured.Server.GitHub.AppID)
	require.False(t, captured.Overview.Enabled)
}
