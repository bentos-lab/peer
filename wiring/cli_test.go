package wiring

import (
	"testing"

	"bentos-backend/config"
	"bentos-backend/domain"
	"github.com/stretchr/testify/require"
)

func TestValidateCLISelection(t *testing.T) {
	tests := []struct {
		name        string
		inputType   domain.ChangeRequestInputProvider
		publishType domain.ChangeRequestPublishType
		wantErr     bool
	}{
		{
			name:        "local print",
			inputType:   domain.ChangeRequestInputProviderLocal,
			publishType: domain.ChangeRequestPublishTypePrint,
			wantErr:     false,
		},
		{
			name:        "local comment",
			inputType:   domain.ChangeRequestInputProviderLocal,
			publishType: domain.ChangeRequestPublishTypeComment,
			wantErr:     true,
		},
		{
			name:        "github print",
			inputType:   domain.ChangeRequestInputProviderGitHub,
			publishType: domain.ChangeRequestPublishTypePrint,
			wantErr:     false,
		},
		{
			name:        "github comment",
			inputType:   domain.ChangeRequestInputProviderGitHub,
			publishType: domain.ChangeRequestPublishTypeComment,
			wantErr:     false,
		},
		{
			name:        "unknown input",
			inputType:   domain.ChangeRequestInputProvider("unknown"),
			publishType: domain.ChangeRequestPublishTypePrint,
			wantErr:     true,
		},
		{
			name:        "unknown publish for github",
			inputType:   domain.ChangeRequestInputProviderGitHub,
			publishType: domain.ChangeRequestPublishType("unknown"),
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCLISelection(tt.inputType, tt.publishType)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestResolveCLILLMConfigUsesConfiguredModelForShortcutWhenFlagMissing(t *testing.T) {
	cfg := config.Config{
		OpenAI: config.OpenAIConfig{
			BaseURL: "gemini",
			Model:   "env-model",
			APIKey:  "env-key",
		},
	}
	llmCfg, err := resolveCLILLMConfig(cfg, CLILLMOptions{
		OpenAIBaseURL: "openai",
	})
	require.NoError(t, err)
	require.Equal(t, "https://api.openai.com/v1", llmCfg.BaseURL)
	require.Equal(t, "env-model", llmCfg.Model)
	require.Equal(t, "env-key", llmCfg.APIKey)
}

func TestResolveCLILLMConfigFullURLRequiresResolvedModel(t *testing.T) {
	cfg := config.Config{
		OpenAI: config.OpenAIConfig{
			BaseURL: "gemini",
			Model:   "",
			APIKey:  "env-key",
		},
	}
	_, err := resolveCLILLMConfig(cfg, CLILLMOptions{
		OpenAIBaseURL: "https://example.com/v1",
	})
	require.Error(t, err)
}

func TestResolveCLILLMConfigUsesFlagOverrides(t *testing.T) {
	cfg := config.Config{
		OpenAI: config.OpenAIConfig{
			BaseURL: "gemini",
			Model:   "env-model",
			APIKey:  "env-key",
		},
	}
	llmCfg, err := resolveCLILLMConfig(cfg, CLILLMOptions{
		OpenAIBaseURL: "https://example.com/v1",
		OpenAIModel:   "flag-model",
		OpenAIAPIKey:  "flag-key",
	})
	require.NoError(t, err)
	require.Equal(t, "https://example.com/v1", llmCfg.BaseURL)
	require.Equal(t, "flag-model", llmCfg.Model)
	require.Equal(t, "flag-key", llmCfg.APIKey)
}

func TestResolveCLILLMConfigRequiresAPIKey(t *testing.T) {
	cfg := config.Config{
		OpenAI: config.OpenAIConfig{
			BaseURL: "gemini",
			Model:   "env-model",
			APIKey:  "",
		},
	}
	_, err := resolveCLILLMConfig(cfg, CLILLMOptions{})
	require.Error(t, err)
}

func TestBuildCLICommandRejectsUnsupportedSelection(t *testing.T) {
	_, err := BuildCLICommand(
		config.Config{},
		CLILLMOptions{},
		domain.ChangeRequestInputProviderLocal,
		domain.ChangeRequestPublishTypeComment,
		"",
	)
	require.Error(t, err)
}

func TestBuildCLICommandBuildsSupportedSelections(t *testing.T) {
	cfg := config.Config{
		OpenAI: config.OpenAIConfig{
			BaseURL: "openai",
			Model:   "gpt-4.1-mini",
			APIKey:  "test-key",
		},
	}

	tests := []struct {
		name        string
		inputType   domain.ChangeRequestInputProvider
		publishType domain.ChangeRequestPublishType
	}{
		{
			name:        "local print",
			inputType:   domain.ChangeRequestInputProviderLocal,
			publishType: domain.ChangeRequestPublishTypePrint,
		},
		{
			name:        "github print",
			inputType:   domain.ChangeRequestInputProviderGitHub,
			publishType: domain.ChangeRequestPublishTypePrint,
		},
		{
			name:        "github comment",
			inputType:   domain.ChangeRequestInputProviderGitHub,
			publishType: domain.ChangeRequestPublishTypeComment,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := BuildCLICommand(cfg, CLILLMOptions{}, tt.inputType, tt.publishType, "")
			require.NoError(t, err)
			require.NotNil(t, cmd)
		})
	}
}

func TestBuildCLICommandRejectsInvalidLogLevel(t *testing.T) {
	cfg := config.Config{
		LogLevel: "invalid",
		OpenAI: config.OpenAIConfig{
			BaseURL: "openai",
			Model:   "gpt-4.1-mini",
			APIKey:  "test-key",
		},
	}

	_, err := BuildCLICommand(cfg, CLILLMOptions{}, domain.ChangeRequestInputProviderLocal, domain.ChangeRequestPublishTypePrint, "")
	require.Error(t, err)
}

func TestResolveLogLevelUsesOverride(t *testing.T) {
	level, err := resolveLogLevel(config.Config{LogLevel: "error"}, "debug")
	require.NoError(t, err)
	require.Equal(t, "debug", string(level))
}

func TestResolveLogLevelRejectsInvalidValue(t *testing.T) {
	_, err := resolveLogLevel(config.Config{LogLevel: "invalid"}, "")
	require.Error(t, err)
}
