package main

import (
	"bytes"
	"context"
	"errors"
	"log"
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
	lastRequest usecase.ChangeRequestRequest
	executed    bool
}

func validCLIConfig() config.Config {
	return config.Config{
		OpenAI: config.OpenAIConfig{
			BaseURL: "gemini",
			Model:   "gemini-2.5-flash-lite",
		},
	}
}

func boolPtr(value bool) *bool {
	return &value
}

func (f *fakeMainReviewUseCase) Execute(_ context.Context, request usecase.ChangeRequestRequest) (usecase.ChangeRequestExecutionResult, error) {
	f.executed = true
	f.lastRequest = request
	return usecase.ChangeRequestExecutionResult{}, nil
}

func TestRunCLISupportsLongAndShortReviewFlags(t *testing.T) {
	fakeUC := &fakeMainReviewUseCase{}
	var capturedOpts wiring.CLILLMOptions
	var capturedInputType domain.ChangeRequestInputProvider
	var capturedPublishType domain.ChangeRequestPublishType

	err := runCLI(
		context.Background(),
		[]string{"-a", "-u", "-c", "a.go,b.go", "--openai-base-url", "openai"},
		func() (config.Config, error) { return validCLIConfig(), nil },
		func(_ config.Config, opts wiring.CLILLMOptions, inputType domain.ChangeRequestInputProvider, publishType domain.ChangeRequestPublishType, _ string) (*cliinbound.Command, error) {
			capturedOpts = opts
			capturedInputType = inputType
			capturedPublishType = publishType
			return cliinbound.NewLocalCommand(fakeUC, nil), nil
		},
	)
	require.NoError(t, err)
	require.True(t, fakeUC.executed)
	require.Equal(t, "true", fakeUC.lastRequest.Metadata[cliinput.MetadataKeyAutoIncludeAll])
	require.Equal(t, "true", fakeUC.lastRequest.Metadata[cliinput.MetadataKeyAutoIncludeUntracked])
	require.Equal(t, "a.go,b.go", fakeUC.lastRequest.Metadata[cliinput.MetadataKeyChangedFiles])
	require.Equal(t, "openai", capturedOpts.OpenAIBaseURL)
	require.Equal(t, domain.ChangeRequestInputProviderLocal, capturedInputType)
	require.Equal(t, domain.ChangeRequestPublishTypePrint, capturedPublishType)
	require.Zero(t, fakeUC.lastRequest.ChangeRequestNumber)
}

func TestRunCLIParsesOpenAIFlagsEqualsAndSpaceForms(t *testing.T) {
	fakeUC := &fakeMainReviewUseCase{}
	var capturedOpts wiring.CLILLMOptions

	err := runCLI(
		context.Background(),
		[]string{"--openai-base-url=openai", "--openai-model", "gpt-4.1-mini", "--openai-api-key=secret"},
		func() (config.Config, error) { return validCLIConfig(), nil },
		func(_ config.Config, opts wiring.CLILLMOptions, _ domain.ChangeRequestInputProvider, _ domain.ChangeRequestPublishType, _ string) (*cliinbound.Command, error) {
			capturedOpts = opts
			return cliinbound.NewLocalCommand(fakeUC, nil), nil
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
		func() (config.Config, error) { return validCLIConfig(), nil },
		func(_ config.Config, _ wiring.CLILLMOptions, _ domain.ChangeRequestInputProvider, _ domain.ChangeRequestPublishType, _ string) (*cliinbound.Command, error) {
			return cliinbound.NewLocalCommand(fakeUC, nil), nil
		},
	)
	require.Error(t, err)
	require.False(t, fakeUC.executed)
}

func TestRunCLIMarksConfigLoadErrors(t *testing.T) {
	fakeUC := &fakeMainReviewUseCase{}
	var buffer bytes.Buffer

	restoreLogging := captureCLILogOutput(t, &buffer)
	defer restoreLogging()

	err := runCLI(
		context.Background(),
		[]string{"--all"},
		func() (config.Config, error) { return config.Config{}, errors.New("config boom") },
		func(_ config.Config, _ wiring.CLILLMOptions, _ domain.ChangeRequestInputProvider, _ domain.ChangeRequestPublishType, _ string) (*cliinbound.Command, error) {
			return cliinbound.NewLocalCommand(fakeUC, nil), nil
		},
	)

	require.Error(t, err)
	require.ErrorContains(t, err, "load config: config boom")
	require.ErrorIs(t, err, errCLIConfigLoad)
	require.Empty(t, buffer.String())
	require.False(t, fakeUC.executed)
}

func TestRunCLIDoesNotOverrideOpenAIEnvWhenFlagsNotProvided(t *testing.T) {
	fakeUC := &fakeMainReviewUseCase{}
	var capturedOpts wiring.CLILLMOptions

	err := runCLI(
		context.Background(),
		[]string{"--all"},
		func() (config.Config, error) { return validCLIConfig(), nil },
		func(_ config.Config, opts wiring.CLILLMOptions, _ domain.ChangeRequestInputProvider, _ domain.ChangeRequestPublishType, _ string) (*cliinbound.Command, error) {
			capturedOpts = opts
			return cliinbound.NewLocalCommand(fakeUC, nil), nil
		},
	)
	require.NoError(t, err)
	require.Equal(t, wiring.CLILLMOptions{}, capturedOpts)
	require.True(t, fakeUC.executed)
}

func TestRunCLISuggestedChangesFlagTrueOverridesEnvFalse(t *testing.T) {
	fakeUC := &fakeMainReviewUseCase{}
	var capturedCfg config.Config

	err := runCLI(
		context.Background(),
		[]string{"--suggested-changes"},
		func() (config.Config, error) {
			cfg := validCLIConfig()
			cfg.SuggestedChanges.Enabled = false
			return cfg, nil
		},
		func(cfg config.Config, _ wiring.CLILLMOptions, _ domain.ChangeRequestInputProvider, _ domain.ChangeRequestPublishType, _ string) (*cliinbound.Command, error) {
			capturedCfg = cfg
			return cliinbound.NewLocalCommand(fakeUC, nil), nil
		},
	)
	require.NoError(t, err)
	require.True(t, fakeUC.executed)
	require.True(t, capturedCfg.SuggestedChanges.Enabled)
}

func TestRunCLISuggestedChangesFlagFalseOverridesEnvTrue(t *testing.T) {
	fakeUC := &fakeMainReviewUseCase{}
	var capturedCfg config.Config

	err := runCLI(
		context.Background(),
		[]string{"--suggested-changes=false"},
		func() (config.Config, error) {
			cfg := validCLIConfig()
			cfg.SuggestedChanges.Enabled = true
			return cfg, nil
		},
		func(cfg config.Config, _ wiring.CLILLMOptions, _ domain.ChangeRequestInputProvider, _ domain.ChangeRequestPublishType, _ string) (*cliinbound.Command, error) {
			capturedCfg = cfg
			return cliinbound.NewLocalCommand(fakeUC, nil), nil
		},
	)
	require.NoError(t, err)
	require.True(t, fakeUC.executed)
	require.False(t, capturedCfg.SuggestedChanges.Enabled)
}

func TestRunCLISuggestedChangesFlagMissingKeepsEnvValue(t *testing.T) {
	tests := []struct {
		name    string
		enabled bool
	}{
		{
			name:    "env_false",
			enabled: false,
		},
		{
			name:    "env_true",
			enabled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeUC := &fakeMainReviewUseCase{}
			var capturedCfg config.Config

			err := runCLI(
				context.Background(),
				[]string{"--all"},
				func() (config.Config, error) {
					cfg := validCLIConfig()
					cfg.SuggestedChanges.Enabled = tt.enabled
					return cfg, nil
				},
				func(cfg config.Config, _ wiring.CLILLMOptions, _ domain.ChangeRequestInputProvider, _ domain.ChangeRequestPublishType, _ string) (*cliinbound.Command, error) {
					capturedCfg = cfg
					return cliinbound.NewLocalCommand(fakeUC, nil), nil
				},
			)
			require.NoError(t, err)
			require.True(t, fakeUC.executed)
			require.Equal(t, tt.enabled, capturedCfg.SuggestedChanges.Enabled)
		})
	}
}

func TestRunCLIParsesGitHubPRFlags(t *testing.T) {
	fakeUC := &fakeMainReviewUseCase{}
	var capturedInputType domain.ChangeRequestInputProvider
	var capturedPublishType domain.ChangeRequestPublishType

	err := runCLI(
		context.Background(),
		[]string{"--gh-pr", "123"},
		func() (config.Config, error) { return validCLIConfig(), nil },
		func(_ config.Config, _ wiring.CLILLMOptions, inputType domain.ChangeRequestInputProvider, publishType domain.ChangeRequestPublishType, _ string) (*cliinbound.Command, error) {
			capturedInputType = inputType
			capturedPublishType = publishType
			return cliinbound.NewGitHubPRCommand(fakeUC, nil), nil
		},
	)
	require.NoError(t, err)
	require.True(t, fakeUC.executed)
	require.Equal(t, domain.ChangeRequestInputProviderGitHub, capturedInputType)
	require.Equal(t, domain.ChangeRequestPublishTypePrint, capturedPublishType)
	require.Equal(t, 123, fakeUC.lastRequest.ChangeRequestNumber)
}

func TestRunCLIAcceptsCommentOnPRWithGitHubPR(t *testing.T) {
	fakeUC := &fakeMainReviewUseCase{}
	var capturedInputType domain.ChangeRequestInputProvider
	var capturedPublishType domain.ChangeRequestPublishType

	err := runCLI(
		context.Background(),
		[]string{"--gh-pr", "123", "--comment-on-pr"},
		func() (config.Config, error) { return validCLIConfig(), nil },
		func(_ config.Config, _ wiring.CLILLMOptions, inputType domain.ChangeRequestInputProvider, publishType domain.ChangeRequestPublishType, _ string) (*cliinbound.Command, error) {
			capturedInputType = inputType
			capturedPublishType = publishType
			return cliinbound.NewGitHubPRCommand(fakeUC, nil), nil
		},
	)
	require.NoError(t, err)
	require.True(t, fakeUC.executed)
	require.Equal(t, domain.ChangeRequestInputProviderGitHub, capturedInputType)
	require.Equal(t, domain.ChangeRequestPublishTypeComment, capturedPublishType)
	require.Equal(t, 123, fakeUC.lastRequest.ChangeRequestNumber)
	require.False(t, fakeUC.lastRequest.EnableOverview)
}

func TestRunCLIAcceptsOverviewWithoutGitHubPR(t *testing.T) {
	fakeUC := &fakeMainReviewUseCase{}

	err := runCLI(
		context.Background(),
		[]string{"--overview"},
		func() (config.Config, error) { return validCLIConfig(), nil },
		func(_ config.Config, _ wiring.CLILLMOptions, _ domain.ChangeRequestInputProvider, _ domain.ChangeRequestPublishType, _ string) (*cliinbound.Command, error) {
			return cliinbound.NewLocalCommand(fakeUC, nil), nil
		},
	)
	require.NoError(t, err)
	require.True(t, fakeUC.executed)
	require.True(t, fakeUC.lastRequest.EnableOverview)
}

func TestRunCLIAcceptsOverviewWithPRCommentMode(t *testing.T) {
	fakeUC := &fakeMainReviewUseCase{}

	err := runCLI(
		context.Background(),
		[]string{"--gh-pr", "123", "--comment-on-pr", "--overview"},
		func() (config.Config, error) { return validCLIConfig(), nil },
		func(_ config.Config, _ wiring.CLILLMOptions, _ domain.ChangeRequestInputProvider, _ domain.ChangeRequestPublishType, _ string) (*cliinbound.Command, error) {
			return cliinbound.NewGitHubPRCommand(fakeUC, nil), nil
		},
	)
	require.NoError(t, err)
	require.True(t, fakeUC.executed)
	require.True(t, fakeUC.lastRequest.EnableOverview)
}

func TestRunCLIUsesOverviewEnvWhenFlagNotProvided(t *testing.T) {
	fakeUC := &fakeMainReviewUseCase{}

	err := runCLI(
		context.Background(),
		[]string{"--all"},
		func() (config.Config, error) {
			cfg := validCLIConfig()
			cfg.OverviewEnabled = boolPtr(true)
			return cfg, nil
		},
		func(_ config.Config, _ wiring.CLILLMOptions, _ domain.ChangeRequestInputProvider, _ domain.ChangeRequestPublishType, _ string) (*cliinbound.Command, error) {
			return cliinbound.NewLocalCommand(fakeUC, nil), nil
		},
	)
	require.NoError(t, err)
	require.True(t, fakeUC.executed)
	require.True(t, fakeUC.lastRequest.EnableOverview)
}

func TestRunCLIOverviewFlagFalseOverridesOverviewEnvTrue(t *testing.T) {
	fakeUC := &fakeMainReviewUseCase{}

	err := runCLI(
		context.Background(),
		[]string{"--all", "--overview=false"},
		func() (config.Config, error) {
			cfg := validCLIConfig()
			cfg.OverviewEnabled = boolPtr(true)
			return cfg, nil
		},
		func(_ config.Config, _ wiring.CLILLMOptions, _ domain.ChangeRequestInputProvider, _ domain.ChangeRequestPublishType, _ string) (*cliinbound.Command, error) {
			return cliinbound.NewLocalCommand(fakeUC, nil), nil
		},
	)
	require.NoError(t, err)
	require.True(t, fakeUC.executed)
	require.False(t, fakeUC.lastRequest.EnableOverview)
}

func TestRunCLIOverviewFlagTrueOverridesOverviewEnvFalse(t *testing.T) {
	fakeUC := &fakeMainReviewUseCase{}

	err := runCLI(
		context.Background(),
		[]string{"--all", "--overview"},
		func() (config.Config, error) {
			cfg := validCLIConfig()
			cfg.OverviewEnabled = boolPtr(false)
			return cfg, nil
		},
		func(_ config.Config, _ wiring.CLILLMOptions, _ domain.ChangeRequestInputProvider, _ domain.ChangeRequestPublishType, _ string) (*cliinbound.Command, error) {
			return cliinbound.NewLocalCommand(fakeUC, nil), nil
		},
	)
	require.NoError(t, err)
	require.True(t, fakeUC.executed)
	require.True(t, fakeUC.lastRequest.EnableOverview)
}

func TestRunCLIRejectsSummaryFlag(t *testing.T) {
	fakeUC := &fakeMainReviewUseCase{}

	err := runCLI(
		context.Background(),
		[]string{"--summary"},
		func() (config.Config, error) { return validCLIConfig(), nil },
		func(_ config.Config, _ wiring.CLILLMOptions, _ domain.ChangeRequestInputProvider, _ domain.ChangeRequestPublishType, _ string) (*cliinbound.Command, error) {
			return cliinbound.NewLocalCommand(fakeUC, nil), nil
		},
	)
	require.Error(t, err)
	require.False(t, fakeUC.executed)
}

func TestRunCLIRejectsCommentOnPRWithoutGitHubPR(t *testing.T) {
	fakeUC := &fakeMainReviewUseCase{}

	err := runCLI(
		context.Background(),
		[]string{"--comment-on-pr"},
		func() (config.Config, error) { return validCLIConfig(), nil },
		func(_ config.Config, _ wiring.CLILLMOptions, _ domain.ChangeRequestInputProvider, _ domain.ChangeRequestPublishType, _ string) (*cliinbound.Command, error) {
			return cliinbound.NewLocalCommand(fakeUC, nil), nil
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
				func() (config.Config, error) { return validCLIConfig(), nil },
				func(_ config.Config, _ wiring.CLILLMOptions, _ domain.ChangeRequestInputProvider, _ domain.ChangeRequestPublishType, _ string) (*cliinbound.Command, error) {
					return cliinbound.NewLocalCommand(fakeUC, nil), nil
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
		func() (config.Config, error) { return validCLIConfig(), nil },
		func(_ config.Config, _ wiring.CLILLMOptions, _ domain.ChangeRequestInputProvider, _ domain.ChangeRequestPublishType, _ string) (*cliinbound.Command, error) {
			return cliinbound.NewLocalCommand(fakeUC, nil), nil
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
			return validCLIConfig(), nil
		},
		func(_ config.Config, _ wiring.CLILLMOptions, _ domain.ChangeRequestInputProvider, _ domain.ChangeRequestPublishType, _ string) (*cliinbound.Command, error) {
			buildCalled = true
			return cliinbound.NewLocalCommand(nil, nil), nil
		},
	)
	require.NoError(t, err)
	require.False(t, loadCalled)
	require.False(t, buildCalled)
}

func TestRunCLIParsesLogLevelFlag(t *testing.T) {
	fakeUC := &fakeMainReviewUseCase{}
	var capturedLogLevel string

	err := runCLI(
		context.Background(),
		[]string{"--log-level", "warning"},
		func() (config.Config, error) { return validCLIConfig(), nil },
		func(_ config.Config, _ wiring.CLILLMOptions, _ domain.ChangeRequestInputProvider, _ domain.ChangeRequestPublishType, logLevelOverride string) (*cliinbound.Command, error) {
			capturedLogLevel = logLevelOverride
			return cliinbound.NewLocalCommand(fakeUC, nil), nil
		},
	)
	require.NoError(t, err)
	require.Equal(t, "warning", capturedLogLevel)
}

func TestRunCLIRejectsInvalidLogLevelFlag(t *testing.T) {
	fakeUC := &fakeMainReviewUseCase{}

	err := runCLI(
		context.Background(),
		[]string{"--log-level", "verbose"},
		func() (config.Config, error) { return validCLIConfig(), nil },
		func(_ config.Config, _ wiring.CLILLMOptions, _ domain.ChangeRequestInputProvider, _ domain.ChangeRequestPublishType, _ string) (*cliinbound.Command, error) {
			return cliinbound.NewLocalCommand(fakeUC, nil), nil
		},
	)
	require.Error(t, err)
	require.False(t, fakeUC.executed)
}

func TestRunCLIOmitsLogLevelOverrideWhenFlagMissing(t *testing.T) {
	fakeUC := &fakeMainReviewUseCase{}
	var capturedLogLevel string

	err := runCLI(
		context.Background(),
		[]string{"--all"},
		func() (config.Config, error) {
			cfg := validCLIConfig()
			cfg.LogLevel = "error"
			return cfg, nil
		},
		func(_ config.Config, _ wiring.CLILLMOptions, _ domain.ChangeRequestInputProvider, _ domain.ChangeRequestPublishType, logLevelOverride string) (*cliinbound.Command, error) {
			capturedLogLevel = logLevelOverride
			return cliinbound.NewLocalCommand(fakeUC, nil), nil
		},
	)
	require.NoError(t, err)
	require.Equal(t, "", capturedLogLevel)
}

func TestRunCLILogsResolvedShortcutLLMConfig(t *testing.T) {
	fakeUC := &fakeMainReviewUseCase{}
	var buffer bytes.Buffer

	restoreLogging := captureCLILogOutput(t, &buffer)
	defer restoreLogging()

	err := runCLI(
		context.Background(),
		[]string{"--all"},
		func() (config.Config, error) { return validCLIConfig(), nil },
		func(_ config.Config, _ wiring.CLILLMOptions, _ domain.ChangeRequestInputProvider, _ domain.ChangeRequestPublishType, _ string) (*cliinbound.Command, error) {
			return cliinbound.NewLocalCommand(fakeUC, nil), nil
		},
	)
	require.NoError(t, err)
	require.Contains(
		t,
		buffer.String(),
		`cli startup: llm_config base_url="https://generativelanguage.googleapis.com/v1beta/openai" model="gemini-2.5-flash-lite"`,
	)
}

func TestRunCLILogsResolvedLLMConfigFromFlags(t *testing.T) {
	fakeUC := &fakeMainReviewUseCase{}
	var buffer bytes.Buffer

	restoreLogging := captureCLILogOutput(t, &buffer)
	defer restoreLogging()

	err := runCLI(
		context.Background(),
		[]string{"--openai-base-url", "openai", "--openai-model", "gpt-4.1"},
		func() (config.Config, error) { return validCLIConfig(), nil },
		func(_ config.Config, _ wiring.CLILLMOptions, _ domain.ChangeRequestInputProvider, _ domain.ChangeRequestPublishType, _ string) (*cliinbound.Command, error) {
			return cliinbound.NewLocalCommand(fakeUC, nil), nil
		},
	)
	require.NoError(t, err)
	require.Contains(
		t,
		buffer.String(),
		`cli startup: llm_config base_url="https://api.openai.com/v1" model="gpt-4.1"`,
	)
}

func TestRunCLILogsResolvedFullURLLLMConfig(t *testing.T) {
	fakeUC := &fakeMainReviewUseCase{}
	var buffer bytes.Buffer

	restoreLogging := captureCLILogOutput(t, &buffer)
	defer restoreLogging()

	err := runCLI(
		context.Background(),
		[]string{"--all"},
		func() (config.Config, error) {
			cfg := validCLIConfig()
			cfg.OpenAI.BaseURL = "https://example.com/v1"
			cfg.OpenAI.Model = "custom-model"
			return cfg, nil
		},
		func(_ config.Config, _ wiring.CLILLMOptions, _ domain.ChangeRequestInputProvider, _ domain.ChangeRequestPublishType, _ string) (*cliinbound.Command, error) {
			return cliinbound.NewLocalCommand(fakeUC, nil), nil
		},
	)
	require.NoError(t, err)
	require.Contains(
		t,
		buffer.String(),
		`cli startup: llm_config base_url="https://example.com/v1" model="custom-model"`,
	)
}

func captureCLILogOutput(t *testing.T, output *bytes.Buffer) func() {
	t.Helper()

	originalWriter := log.Writer()
	originalFlags := log.Flags()
	originalPrefix := log.Prefix()

	log.SetOutput(output)
	log.SetFlags(0)
	log.SetPrefix("")

	return func() {
		log.SetOutput(originalWriter)
		log.SetFlags(originalFlags)
		log.SetPrefix(originalPrefix)
	}
}
