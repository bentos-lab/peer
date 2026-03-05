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
	unsetEnv(t, "GITHUB_WEBHOOK_SECRET")
	unsetEnv(t, "GITHUB_APP_ID")
	unsetEnv(t, "GITHUB_APP_PRIVATE_KEY")
	unsetEnv(t, "GITHUB_API_BASE_URL")

	tmp := t.TempDir()
	originalWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmp))
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})

	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, "info", cfg.LogLevel)
	require.Equal(t, "gemini", cfg.OpenAI.BaseURL)
	require.Equal(t, "gemini-2.5-flash-lite", cfg.OpenAI.Model)
	require.Equal(t, "8080", cfg.Server.Port)
	require.Equal(t, "", cfg.Server.GitHub.WebhookSecret)
	require.Equal(t, "", cfg.Server.GitHub.AppID)
	require.Equal(t, "", cfg.Server.GitHub.AppPrivateKey)
	require.Equal(t, "https://api.github.com", cfg.Server.GitHub.APIBaseURL)
}

func TestLoadReadsDotEnvWhenEnvMissing(t *testing.T) {
	unsetEnv(t, "OPENAI_BASE_URL")
	unsetEnv(t, "OPENAI_MODEL")
	unsetEnv(t, "OPENAI_API_KEY")
	unsetEnv(t, "PORT")
	unsetEnv(t, "LOG_LEVEL")
	unsetEnv(t, "GITHUB_WEBHOOK_SECRET")
	unsetEnv(t, "GITHUB_APP_ID")
	unsetEnv(t, "GITHUB_APP_PRIVATE_KEY")
	unsetEnv(t, "GITHUB_API_BASE_URL")

	tmp := t.TempDir()
	originalWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmp))
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})

	envPath := filepath.Join(tmp, ".env")
	require.NoError(t, os.WriteFile(envPath, []byte("OPENAI_BASE_URL=openai\nOPENAI_MODEL=my-model\nOPENAI_API_KEY=env-key\nPORT=9090\nLOG_LEVEL=warning\nGITHUB_WEBHOOK_SECRET=whsec\nGITHUB_APP_ID=12345\nGITHUB_APP_PRIVATE_KEY=-----BEGIN PRIVATE KEY-----\\nabc\\n-----END PRIVATE KEY-----\nGITHUB_API_BASE_URL=https://github.example.com/api/v3\n"), 0o644))

	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, "openai", cfg.OpenAI.BaseURL)
	require.Equal(t, "my-model", cfg.OpenAI.Model)
	require.Equal(t, "env-key", cfg.OpenAI.APIKey)
	require.Equal(t, "warning", cfg.LogLevel)
	require.Equal(t, "9090", cfg.Server.Port)
	require.Equal(t, "whsec", cfg.Server.GitHub.WebhookSecret)
	require.Equal(t, "12345", cfg.Server.GitHub.AppID)
	require.Equal(t, "-----BEGIN PRIVATE KEY-----\\nabc\\n-----END PRIVATE KEY-----", cfg.Server.GitHub.AppPrivateKey)
	require.Equal(t, "https://github.example.com/api/v3", cfg.Server.GitHub.APIBaseURL)
}

func TestLoadDoesNotOverrideExistingEnvWithDotEnv(t *testing.T) {
	t.Setenv("OPENAI_BASE_URL", "gemini")
	t.Setenv("OPENAI_MODEL", "from-env")
	t.Setenv("OPENAI_API_KEY", "env-key")
	t.Setenv("PORT", "7777")
	t.Setenv("LOG_LEVEL", "error")
	t.Setenv("GITHUB_WEBHOOK_SECRET", "set-secret")
	t.Setenv("GITHUB_APP_ID", "set-app-id")
	t.Setenv("GITHUB_APP_PRIVATE_KEY", "set-private-key")
	t.Setenv("GITHUB_API_BASE_URL", "https://set.example.com")

	tmp := t.TempDir()
	originalWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmp))
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})

	envPath := filepath.Join(tmp, ".env")
	require.NoError(t, os.WriteFile(envPath, []byte("OPENAI_BASE_URL=openai\nOPENAI_MODEL=from-dotenv\nOPENAI_API_KEY=dotenv-key\nPORT=9090\nLOG_LEVEL=warning\nGITHUB_WEBHOOK_SECRET=dotenv-secret\nGITHUB_APP_ID=dotenv-app-id\nGITHUB_APP_PRIVATE_KEY=dotenv-private-key\nGITHUB_API_BASE_URL=https://dotenv.example.com\n"), 0o644))

	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, "gemini", cfg.OpenAI.BaseURL)
	require.Equal(t, "from-env", cfg.OpenAI.Model)
	require.Equal(t, "env-key", cfg.OpenAI.APIKey)
	require.Equal(t, "error", cfg.LogLevel)
	require.Equal(t, "7777", cfg.Server.Port)
	require.Equal(t, "set-secret", cfg.Server.GitHub.WebhookSecret)
	require.Equal(t, "set-app-id", cfg.Server.GitHub.AppID)
	require.Equal(t, "set-private-key", cfg.Server.GitHub.AppPrivateKey)
	require.Equal(t, "https://set.example.com", cfg.Server.GitHub.APIBaseURL)
}

func TestLoadReturnsErrorForInvalidDotEnv(t *testing.T) {
	unsetEnv(t, "OPENAI_BASE_URL")
	unsetEnv(t, "OPENAI_MODEL")
	unsetEnv(t, "OPENAI_API_KEY")
	unsetEnv(t, "PORT")
	unsetEnv(t, "LOG_LEVEL")
	unsetEnv(t, "GITHUB_WEBHOOK_SECRET")
	unsetEnv(t, "GITHUB_APP_ID")
	unsetEnv(t, "GITHUB_APP_PRIVATE_KEY")
	unsetEnv(t, "GITHUB_API_BASE_URL")

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
