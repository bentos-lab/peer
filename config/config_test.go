package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadUsesDefaultsWhenNoEnv(t *testing.T) {
	unsetEnv(t, "PORT")
	unsetEnv(t, "LOG_LEVEL")
	unsetEnv(t, "OPENAI_BASE_URL")
	unsetEnv(t, "OPENAI_API_KEY")
	unsetEnv(t, "OPENAI_MODEL")

	tmp := t.TempDir()
	originalWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmp))
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})

	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, "8080", cfg.Port)
	require.Equal(t, "info", cfg.LogLevel)
	require.Equal(t, "gemini", cfg.OpenAIBaseURL)
	require.Equal(t, "gemini-2.5-flash-lite", cfg.OpenAIModel)
}

func TestLoadReadsDotEnvWhenEnvMissing(t *testing.T) {
	unsetEnv(t, "OPENAI_BASE_URL")
	unsetEnv(t, "OPENAI_MODEL")
	unsetEnv(t, "OPENAI_API_KEY")
	unsetEnv(t, "PORT")
	unsetEnv(t, "LOG_LEVEL")

	tmp := t.TempDir()
	originalWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmp))
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})

	envPath := filepath.Join(tmp, ".env")
	require.NoError(t, os.WriteFile(envPath, []byte("OPENAI_BASE_URL=openai\nOPENAI_MODEL=my-model\nOPENAI_API_KEY=env-key\nPORT=9090\nLOG_LEVEL=warning\n"), 0o644))

	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, "openai", cfg.OpenAIBaseURL)
	require.Equal(t, "my-model", cfg.OpenAIModel)
	require.Equal(t, "env-key", cfg.OpenAIAPIKey)
	require.Equal(t, "9090", cfg.Port)
	require.Equal(t, "warning", cfg.LogLevel)
}

func TestLoadDoesNotOverrideExistingEnvWithDotEnv(t *testing.T) {
	t.Setenv("OPENAI_BASE_URL", "gemini")
	t.Setenv("OPENAI_MODEL", "from-env")
	t.Setenv("OPENAI_API_KEY", "env-key")
	t.Setenv("PORT", "7777")
	t.Setenv("LOG_LEVEL", "error")

	tmp := t.TempDir()
	originalWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmp))
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})

	envPath := filepath.Join(tmp, ".env")
	require.NoError(t, os.WriteFile(envPath, []byte("OPENAI_BASE_URL=openai\nOPENAI_MODEL=from-dotenv\nOPENAI_API_KEY=dotenv-key\nPORT=9090\nLOG_LEVEL=warning\n"), 0o644))

	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, "gemini", cfg.OpenAIBaseURL)
	require.Equal(t, "from-env", cfg.OpenAIModel)
	require.Equal(t, "env-key", cfg.OpenAIAPIKey)
	require.Equal(t, "7777", cfg.Port)
	require.Equal(t, "error", cfg.LogLevel)
}

func TestLoadReturnsErrorForInvalidDotEnv(t *testing.T) {
	unsetEnv(t, "OPENAI_BASE_URL")
	unsetEnv(t, "OPENAI_MODEL")
	unsetEnv(t, "OPENAI_API_KEY")
	unsetEnv(t, "PORT")
	unsetEnv(t, "LOG_LEVEL")

	tmp := t.TempDir()
	originalWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmp))
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})

	envPath := filepath.Join(tmp, ".env")
	require.NoError(t, os.WriteFile(envPath, []byte("OPENAI_BASE_URL=\"unterminated\n"), 0o644))

	_, err = Load()
	require.Error(t, err)
}

func unsetEnv(t *testing.T, key string) {
	t.Helper()
	oldValue, hadValue := os.LookupEnv(key)
	require.NoError(t, os.Unsetenv(key))
	t.Cleanup(func() {
		if hadValue {
			_ = os.Setenv(key, oldValue)
			return
		}
		_ = os.Unsetenv(key)
	})
}
