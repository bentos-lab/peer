package main

import (
	"context"
	"testing"

	cliinbound "bentos-backend/adapter/inbound/cli"
	cliinput "bentos-backend/adapter/outbound/input/cli"
	"bentos-backend/config"
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

	err := runCLI(
		context.Background(),
		[]string{"-a", "-u", "-c", "a.go,b.go", "--openai-base-url", "openai"},
		func() (config.Config, error) { return config.Config{}, nil },
		func(_ config.Config, opts wiring.CLILLMOptions) (*cliinbound.Command, error) {
			capturedOpts = opts
			return cliinbound.NewCommand(fakeUC), nil
		},
	)
	require.NoError(t, err)
	require.True(t, fakeUC.executed)
	require.Equal(t, "true", fakeUC.lastRequest.Metadata[cliinput.MetadataKeyAutoIncludeAll])
	require.Equal(t, "true", fakeUC.lastRequest.Metadata[cliinput.MetadataKeyAutoIncludeUntracked])
	require.Equal(t, "a.go,b.go", fakeUC.lastRequest.Metadata[cliinput.MetadataKeyChangedFiles])
	require.Equal(t, "openai", capturedOpts.OpenAIBaseURL)
}

func TestRunCLIParsesOpenAIFlagsEqualsAndSpaceForms(t *testing.T) {
	fakeUC := &fakeMainReviewUseCase{}
	var capturedOpts wiring.CLILLMOptions

	err := runCLI(
		context.Background(),
		[]string{"--openai-base-url=openai", "--openai-model", "gpt-4.1-mini", "--openai-api-key=secret"},
		func() (config.Config, error) { return config.Config{}, nil },
		func(_ config.Config, opts wiring.CLILLMOptions) (*cliinbound.Command, error) {
			capturedOpts = opts
			return cliinbound.NewCommand(fakeUC), nil
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
		func(_ config.Config, _ wiring.CLILLMOptions) (*cliinbound.Command, error) {
			return cliinbound.NewCommand(fakeUC), nil
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
		func(_ config.Config, opts wiring.CLILLMOptions) (*cliinbound.Command, error) {
			capturedOpts = opts
			return cliinbound.NewCommand(fakeUC), nil
		},
	)
	require.NoError(t, err)
	require.Equal(t, wiring.CLILLMOptions{}, capturedOpts)
	require.True(t, fakeUC.executed)
}

func TestRunCLIRejectsUnknownRemovedFlags(t *testing.T) {
	fakeUC := &fakeMainReviewUseCase{}

	err := runCLI(
		context.Background(),
		[]string{"--base", "main"},
		func() (config.Config, error) { return config.Config{}, nil },
		func(_ config.Config, _ wiring.CLILLMOptions) (*cliinbound.Command, error) {
			return cliinbound.NewCommand(fakeUC), nil
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
		func(_ config.Config, _ wiring.CLILLMOptions) (*cliinbound.Command, error) {
			buildCalled = true
			return cliinbound.NewCommand(nil), nil
		},
	)
	require.NoError(t, err)
	require.False(t, loadCalled)
	require.False(t, buildCalled)
}
