package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"testing"

	cliinbound "github.com/bentos-lab/peer/adapter/inbound/cli"
	gitlabinbound "github.com/bentos-lab/peer/adapter/inbound/http/gitlab"
	"github.com/bentos-lab/peer/config"
	"github.com/bentos-lab/peer/domain"
	"github.com/bentos-lab/peer/usecase"
	uccontracts "github.com/bentos-lab/peer/usecase/contracts"
	"github.com/bentos-lab/peer/wiring"
	"github.com/stretchr/testify/require"
)

const (
	testVersion = "test-version"
	testCommit  = "test-commit"
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

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	original := os.Stderr
	os.Stderr = writer

	done := make(chan string, 1)
	go func() {
		var buffer bytes.Buffer
		_, _ = io.Copy(&buffer, reader)
		_ = reader.Close()
		done <- buffer.String()
	}()

	fn()

	_ = writer.Close()
	os.Stderr = original
	return <-done
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

func minimalPeerDeps() peerDeps {
	return peerDeps{
		loadConfig: func() (config.Config, error) {
			return config.Config{}, nil
		},
		buildReviewCommand: func(_ config.Config, _ wiring.CLILLMOptions, _ string) (*cliinbound.ReviewCommand, error) {
			return nil, nil
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

			deps := peerDeps{
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

			err := runPeer(context.Background(), args, deps, testVersion, testCommit)
			require.NoError(t, err)
			require.Len(t, reviewUseCase.requests, 1)
			require.Equal(t, testCase.expectedSuggest, reviewUseCase.requests[0].Suggestions)
		})
	}
}

func TestRunCLIOverviewSubcommandForcesOverviewOnly(t *testing.T) {
	overviewUseCase := &mainTestOverviewUseCase{}
	githubClient := &mainTestGitHubClient{}

	deps := peerDeps{
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

	err := runPeer(context.Background(), []string{"overview", "--vcs-provider", "github"}, deps, testVersion, testCommit)
	require.NoError(t, err)
	require.Len(t, overviewUseCase.requests, 1)
}

func TestCLIAutoDetectVCSProviderFromRepo(t *testing.T) {
	reviewUseCase := &mainTestReviewUseCase{}
	githubClient := &mainTestGitHubClient{}

	deps := peerDeps{
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
	err := runPeer(context.Background(), args, deps, testVersion, testCommit)
	require.NoError(t, err)
	require.Len(t, reviewUseCase.requests, 1)
}

func TestCLIAutoDetectVCSProviderFromOriginURL(t *testing.T) {
	overviewUseCase := &mainTestOverviewUseCase{}
	gitlabClient := &mainTestGitLabClient{}

	deps := peerDeps{
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

	err := runPeer(context.Background(), []string{"overview"}, deps, testVersion, testCommit)
	require.NoError(t, err)
	require.Len(t, overviewUseCase.requests, 1)
}

func TestRunCLIOverviewIssueAlignmentFlag(t *testing.T) {
	overviewUseCase := &mainTestOverviewUseCase{}
	githubClient := &mainTestGitHubClient{}

	deps := peerDeps{
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

	err := runPeer(context.Background(), []string{"overview", "--issue-alignment", "--change-request", "7", "--vcs-provider", "github"}, deps, testVersion, testCommit)
	require.NoError(t, err)
	require.Len(t, overviewUseCase.requests, 1)
	require.NotEmpty(t, overviewUseCase.requests[0].IssueAlignment.Candidates)
}

func TestWebhookRequiresProvider(t *testing.T) {
	deps := peerDeps{
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

	err := runPeer(context.Background(), []string{"webhook"}, deps, testVersion, testCommit)
	require.Error(t, err)
}

func TestWebhookRejectsUnsupportedProvider(t *testing.T) {
	deps := peerDeps{
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

	err := runPeer(context.Background(), []string{"webhook", "--vcs-provider", "bitbucket"}, deps, testVersion, testCommit)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported vcs provider")
}

func TestWebhookOverridesConfig(t *testing.T) {
	var captured config.Config
	deps := peerDeps{
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
					TriggerName: "peerbot",
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
	err := runPeer(context.Background(), args, deps, testVersion, testCommit)
	require.NoError(t, err)
	require.Equal(t, "999", captured.Server.GitHub.AppID)
	require.False(t, captured.Overview.Enabled)
}

func TestCLIInvalidFlagDoesNotPrintUsage(t *testing.T) {
	deps := minimalPeerDeps()
	stderr := captureStderr(t, func() {
		err := runPeer(context.Background(), []string{"overview", "--bad-flag"}, deps, testVersion, testCommit)
		require.Error(t, err)
	})
	require.Contains(t, stderr, "Error: unknown flag")
	require.NotContains(t, stderr, "Usage:")
}

func TestCLIMissingRequiredFlagsDoesNotPrintUsage(t *testing.T) {
	deps := minimalPeerDeps()
	stderr := captureStderr(t, func() {
		err := runPeer(context.Background(), []string{"webhook"}, deps, testVersion, testCommit)
		require.Error(t, err)
	})
	require.Contains(t, stderr, "Error: at least one vcs provider is required")
	require.NotContains(t, stderr, "Usage:")
}

func TestCLIUnknownSubcommandDoesNotPrintUsage(t *testing.T) {
	deps := minimalPeerDeps()
	stderr := captureStderr(t, func() {
		err := runPeer(context.Background(), []string{"unknown-subcommand"}, deps, testVersion, testCommit)
		require.Error(t, err)
	})
	require.Contains(t, stderr, "Error: unknown command")
	require.NotContains(t, stderr, "Usage:")
}
