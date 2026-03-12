package main

import (
	"context"
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

			err := runCLI(
				context.Background(),
				args,
				func() (config.Config, error) {
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
				func(_ config.Config, _ wiring.CLILLMOptions, _ string) (*cliinbound.ReviewCommand, error) {
					builder := func(_ string) (usecase.ChangeRequestUseCase, error) {
						return changeRequestUseCase, nil
					}
					return cliinbound.NewReviewCommand(builder, githubClient, nil), nil
				},
				func(_ config.Config, _ wiring.CLILLMOptions, _ string) (*cliinbound.OverviewCommand, error) {
					return nil, nil
				},
				func(_ config.Config, _ wiring.CLILLMOptions, _ string) (*cliinbound.AutogenCommand, error) {
					return nil, nil
				},
				func(_ config.Config, _ wiring.CLILLMOptions, _ string) (*cliinbound.ReplyCommentCommand, error) {
					return nil, nil
				},
			)
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

	err := runCLI(
		context.Background(),
		[]string{"overview"},
		func() (config.Config, error) {
			return config.Config{
				LogLevel: "info",
				CodingAgent: config.CodingAgentConfig{
					Agent: "opencode",
				},
			}, nil
		},
		func(_ config.Config, _ wiring.CLILLMOptions, _ string) (*cliinbound.ReviewCommand, error) {
			builder := func(_ string) (usecase.ChangeRequestUseCase, error) {
				return changeRequestUseCase, nil
			}
			return cliinbound.NewReviewCommand(builder, githubClient, nil), nil
		},
		func(_ config.Config, _ wiring.CLILLMOptions, _ string) (*cliinbound.OverviewCommand, error) {
			builder := func(_ string) (usecase.ChangeRequestUseCase, error) {
				return changeRequestUseCase, nil
			}
			return cliinbound.NewOverviewCommand(builder, githubClient, nil), nil
		},
		func(_ config.Config, _ wiring.CLILLMOptions, _ string) (*cliinbound.AutogenCommand, error) {
			return nil, nil
		},
		func(_ config.Config, _ wiring.CLILLMOptions, _ string) (*cliinbound.ReplyCommentCommand, error) {
			return nil, nil
		},
	)
	require.NoError(t, err)
	require.Len(t, changeRequestUseCase.requests, 1)
	require.False(t, changeRequestUseCase.requests[0].EnableReview)
	require.True(t, changeRequestUseCase.requests[0].EnableOverview)
	require.True(t, changeRequestUseCase.requests[0].ReviewExplicit)
	require.True(t, changeRequestUseCase.requests[0].OverviewExplicit)
}
