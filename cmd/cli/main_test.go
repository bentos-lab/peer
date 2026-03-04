package main

import (
	"context"
	"testing"

	cliinbound "bentos-backend/adapter/inbound/cli"
	cliinput "bentos-backend/adapter/outbound/input/cli"
	"bentos-backend/config"
	"bentos-backend/domain"
	"bentos-backend/usecase"
	"bentos-backend/wiring"
	"github.com/stretchr/testify/require"
)

type fakeMainReviewUseCase struct {
	lastRequest usecase.ReviewRequest
	executed    bool
}

func (f *fakeMainReviewUseCase) Execute(_ context.Context, request usecase.ReviewRequest) (usecase.ReviewExecutionResult, error) {
	f.executed = true
	f.lastRequest = request
	return usecase.ReviewExecutionResult{}, nil
}

func TestRunCLISupportsLongAndShortReviewFlags(t *testing.T) {
	fakeUC := &fakeMainReviewUseCase{}
	var capturedOpts wiring.CLILLMOptions
	var capturedInputType domain.ReviewInputProvider
	var capturedPublishType domain.ReviewPublishType

	err := runCLI(
		context.Background(),
		[]string{"-a", "-u", "-c", "a.go,b.go", "--openai-base-url", "openai"},
		func() (config.Config, error) { return config.Config{}, nil },
		func(_ config.Config, opts wiring.CLILLMOptions, inputType domain.ReviewInputProvider, publishType domain.ReviewPublishType) (*cliinbound.Command, error) {
			capturedOpts = opts
			capturedInputType = inputType
			capturedPublishType = publishType
			return cliinbound.NewLocalCommand(fakeUC), nil
		},
	)
	require.NoError(t, err)
	require.True(t, fakeUC.executed)
	require.Equal(t, "true", fakeUC.lastRequest.Metadata[cliinput.MetadataKeyAutoIncludeAll])
	require.Equal(t, "true", fakeUC.lastRequest.Metadata[cliinput.MetadataKeyAutoIncludeUntracked])
	require.Equal(t, "a.go,b.go", fakeUC.lastRequest.Metadata[cliinput.MetadataKeyChangedFiles])
	require.Equal(t, "openai", capturedOpts.OpenAIBaseURL)
	require.Equal(t, domain.ReviewInputProviderLocal, capturedInputType)
	require.Equal(t, domain.ReviewPublishTypePrint, capturedPublishType)
	require.Zero(t, fakeUC.lastRequest.ChangeRequestNumber)
}

func TestRunCLIParsesOpenAIFlagsEqualsAndSpaceForms(t *testing.T) {
	fakeUC := &fakeMainReviewUseCase{}
	var capturedOpts wiring.CLILLMOptions

	err := runCLI(
		context.Background(),
		[]string{"--openai-base-url=openai", "--openai-model", "gpt-4.1-mini", "--openai-api-key=secret"},
		func() (config.Config, error) { return config.Config{}, nil },
		func(_ config.Config, opts wiring.CLILLMOptions, _ domain.ReviewInputProvider, _ domain.ReviewPublishType) (*cliinbound.Command, error) {
			capturedOpts = opts
			return cliinbound.NewLocalCommand(fakeUC), nil
		},
	)
	require.NoError(t, err)
	require.Equal(t, "openai", capturedOpts.OpenAIBaseURL)
	require.Equal(t, "gpt-4.1-mini", capturedOpts.OpenAIModel)
	require.Equal(t, "secret", capturedOpts.OpenAIAPIKey)
}

func TestRunCLIRejectsMissingOpenAIStringFlagValue(t *testing.T) {
	fakeUC := &fakeMainReviewUseCase{}

	err := runCLI(
		context.Background(),
		[]string{"--openai-model", "--all"},
		func() (config.Config, error) { return config.Config{}, nil },
		func(_ config.Config, _ wiring.CLILLMOptions, _ domain.ReviewInputProvider, _ domain.ReviewPublishType) (*cliinbound.Command, error) {
			return cliinbound.NewLocalCommand(fakeUC), nil
		},
	)
	require.Error(t, err)
	require.False(t, fakeUC.executed)
}

func TestRunCLIDoesNotOverrideOpenAIEnvWhenFlagsNotProvided(t *testing.T) {
	fakeUC := &fakeMainReviewUseCase{}
	var capturedOpts wiring.CLILLMOptions

	err := runCLI(
		context.Background(),
		[]string{"--all"},
		func() (config.Config, error) { return config.Config{}, nil },
		func(_ config.Config, opts wiring.CLILLMOptions, _ domain.ReviewInputProvider, _ domain.ReviewPublishType) (*cliinbound.Command, error) {
			capturedOpts = opts
			return cliinbound.NewLocalCommand(fakeUC), nil
		},
	)
	require.NoError(t, err)
	require.Equal(t, wiring.CLILLMOptions{}, capturedOpts)
	require.True(t, fakeUC.executed)
}

func TestRunCLIParsesGitHubPRFlags(t *testing.T) {
	fakeUC := &fakeMainReviewUseCase{}
	var capturedInputType domain.ReviewInputProvider
	var capturedPublishType domain.ReviewPublishType

	err := runCLI(
		context.Background(),
		[]string{"--gh-pr", "123"},
		func() (config.Config, error) { return config.Config{}, nil },
		func(_ config.Config, _ wiring.CLILLMOptions, inputType domain.ReviewInputProvider, publishType domain.ReviewPublishType) (*cliinbound.Command, error) {
			capturedInputType = inputType
			capturedPublishType = publishType
			return cliinbound.NewGitHubPRCommand(fakeUC), nil
		},
	)
	require.NoError(t, err)
	require.True(t, fakeUC.executed)
	require.Equal(t, domain.ReviewInputProviderGitHub, capturedInputType)
	require.Equal(t, domain.ReviewPublishTypePrint, capturedPublishType)
	require.Equal(t, 123, fakeUC.lastRequest.ChangeRequestNumber)
}

func TestRunCLIAcceptsCommentOnPRWithGitHubPR(t *testing.T) {
	fakeUC := &fakeMainReviewUseCase{}
	var capturedInputType domain.ReviewInputProvider
	var capturedPublishType domain.ReviewPublishType

	err := runCLI(
		context.Background(),
		[]string{"--gh-pr", "123", "--comment-on-pr"},
		func() (config.Config, error) { return config.Config{}, nil },
		func(_ config.Config, _ wiring.CLILLMOptions, inputType domain.ReviewInputProvider, publishType domain.ReviewPublishType) (*cliinbound.Command, error) {
			capturedInputType = inputType
			capturedPublishType = publishType
			return cliinbound.NewGitHubPRCommand(fakeUC), nil
		},
	)
	require.NoError(t, err)
	require.True(t, fakeUC.executed)
	require.Equal(t, domain.ReviewInputProviderGitHub, capturedInputType)
	require.Equal(t, domain.ReviewPublishTypeComment, capturedPublishType)
	require.Equal(t, 123, fakeUC.lastRequest.ChangeRequestNumber)
}

func TestRunCLIRejectsCommentOnPRWithoutGitHubPR(t *testing.T) {
	fakeUC := &fakeMainReviewUseCase{}

	err := runCLI(
		context.Background(),
		[]string{"--comment-on-pr"},
		func() (config.Config, error) { return config.Config{}, nil },
		func(_ config.Config, _ wiring.CLILLMOptions, _ domain.ReviewInputProvider, _ domain.ReviewPublishType) (*cliinbound.Command, error) {
			return cliinbound.NewLocalCommand(fakeUC), nil
		},
	)
	require.Error(t, err)
	require.False(t, fakeUC.executed)
}

func TestRunCLIRejectsLocalSelectionFlagsWhenGitHubPRIsEnabled(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "all",
			args: []string{"--gh-pr", "11", "--all"},
		},
		{
			name: "untracked",
			args: []string{"--gh-pr", "11", "--untracked"},
		},
		{
			name: "changed-files",
			args: []string{"--gh-pr", "11", "--changed-files", "a.go"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeUC := &fakeMainReviewUseCase{}
			err := runCLI(
				context.Background(),
				tt.args,
				func() (config.Config, error) { return config.Config{}, nil },
				func(_ config.Config, _ wiring.CLILLMOptions, _ domain.ReviewInputProvider, _ domain.ReviewPublishType) (*cliinbound.Command, error) {
					return cliinbound.NewLocalCommand(fakeUC), nil
				},
			)
			require.Error(t, err)
			require.False(t, fakeUC.executed)
		})
	}
}

func TestRunCLIRejectsUnknownRemovedFlags(t *testing.T) {
	fakeUC := &fakeMainReviewUseCase{}

	err := runCLI(
		context.Background(),
		[]string{"--base", "main"},
		func() (config.Config, error) { return config.Config{}, nil },
		func(_ config.Config, _ wiring.CLILLMOptions, _ domain.ReviewInputProvider, _ domain.ReviewPublishType) (*cliinbound.Command, error) {
			return cliinbound.NewLocalCommand(fakeUC), nil
		},
	)
	require.Error(t, err)
	require.False(t, fakeUC.executed)
}

func TestRunCLIHelpDoesNotLoadOrExecute(t *testing.T) {
	loadCalled := false
	buildCalled := false

	err := runCLI(
		context.Background(),
		[]string{"--help"},
		func() (config.Config, error) {
			loadCalled = true
			return config.Config{}, nil
		},
		func(_ config.Config, _ wiring.CLILLMOptions, _ domain.ReviewInputProvider, _ domain.ReviewPublishType) (*cliinbound.Command, error) {
			buildCalled = true
			return cliinbound.NewLocalCommand(nil), nil
		},
	)
	require.NoError(t, err)
	require.False(t, loadCalled)
	require.False(t, buildCalled)
}
