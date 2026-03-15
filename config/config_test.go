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
	unsetEnv(t, "OVERVIEW_ENABLED")
	unsetEnv(t, "LLM_OPENAI_BASE_URL")
	unsetEnv(t, "LLM_OPENAI_API_KEY")
	unsetEnv(t, "LLM_OPENAI_MODEL")
	unsetEnv(t, "CODING_AGENT_NAME")
	unsetEnv(t, "CODING_AGENT_PROVIDER")
	unsetEnv(t, "CODING_AGENT_MODEL")
	unsetEnv(t, "GITHUB_WEBHOOK_SECRET")
	unsetEnv(t, "GITHUB_APP_ID")
	unsetEnv(t, "GITHUB_APP_PRIVATE_KEY")
	unsetEnv(t, "GITHUB_API_BASE_URL")
	unsetEnv(t, "REPLYCOMMENT_TRIGGER_NAME")
	unsetEnv(t, "REVIEW_SUGGESTED_CHANGES")

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
	require.Nil(t, cfg.OverviewEnabled)
	require.Equal(t, "", cfg.OpenAI.BaseURL)
	require.Equal(t, "", cfg.OpenAI.APIKey)
	require.Equal(t, "", cfg.OpenAI.Model)
	require.Equal(t, "opencode", cfg.CodingAgent.Agent)
	require.Equal(t, "", cfg.CodingAgent.Provider)
	require.Equal(t, "", cfg.CodingAgent.Model)
	require.Equal(t, "8080", cfg.Server.Port)
	require.Equal(t, "", cfg.Server.GitHub.WebhookSecret)
	require.Equal(t, "", cfg.Server.GitHub.AppID)
	require.Equal(t, "", cfg.Server.GitHub.AppPrivateKey)
	require.Equal(t, "https://api.github.com", cfg.Server.GitHub.APIBaseURL)
	require.Equal(t, "autogitbot", cfg.Server.GitHub.ReplyCommentTriggerName)
	require.False(t, cfg.SuggestedChanges.Enabled)
	require.Equal(t, "MAJOR", cfg.SuggestedChanges.MinSeverity)
	require.Equal(t, 50, cfg.SuggestedChanges.MaxCandidates)
	require.Equal(t, 5, cfg.SuggestedChanges.MaxGroupSize)
	require.Equal(t, 3, cfg.SuggestedChanges.MaxWorkers)
	require.Equal(t, 20000, cfg.SuggestedChanges.GroupTimeoutMS)
	require.Equal(t, 30000, cfg.SuggestedChanges.GenerateTimeoutMS)
}

func TestLoadReadsDotEnvWhenEnvMissing(t *testing.T) {
	unsetEnv(t, "LLM_OPENAI_BASE_URL")
	unsetEnv(t, "LLM_OPENAI_API_KEY")
	unsetEnv(t, "LLM_OPENAI_MODEL")
	unsetEnv(t, "CODING_AGENT_NAME")
	unsetEnv(t, "CODING_AGENT_PROVIDER")
	unsetEnv(t, "CODING_AGENT_MODEL")
	unsetEnv(t, "PORT")
	unsetEnv(t, "LOG_LEVEL")
	unsetEnv(t, "OVERVIEW_ENABLED")
	unsetEnv(t, "GITHUB_WEBHOOK_SECRET")
	unsetEnv(t, "GITHUB_APP_ID")
	unsetEnv(t, "GITHUB_APP_PRIVATE_KEY")
	unsetEnv(t, "GITHUB_API_BASE_URL")
	unsetEnv(t, "REPLYCOMMENT_TRIGGER_NAME")
	unsetEnv(t, "REVIEW_SUGGESTED_CHANGES")

	tmp := t.TempDir()
	originalWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmp))
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})

	envPath := filepath.Join(tmp, ".env")
	require.NoError(t, os.WriteFile(envPath, []byte("LLM_OPENAI_BASE_URL=openai\nLLM_OPENAI_MODEL=gpt-4.1\nLLM_OPENAI_API_KEY=env-key\nCODING_AGENT_NAME=opencode\nCODING_AGENT_PROVIDER=codingagent\nCODING_AGENT_MODEL=model-x\nPORT=9090\nLOG_LEVEL=warning\nGITHUB_WEBHOOK_SECRET=whsec\nGITHUB_APP_ID=12345\nGITHUB_APP_PRIVATE_KEY=-----BEGIN PRIVATE KEY-----\\nabc\\n-----END PRIVATE KEY-----\nGITHUB_API_BASE_URL=https://github.example.com/api/v3\nREPLYCOMMENT_TRIGGER_NAME=autogit\n"), 0o644))

	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, "openai", cfg.OpenAI.BaseURL)
	require.Equal(t, "gpt-4.1", cfg.OpenAI.Model)
	require.Equal(t, "env-key", cfg.OpenAI.APIKey)
	require.Equal(t, "opencode", cfg.CodingAgent.Agent)
	require.Equal(t, "codingagent", cfg.CodingAgent.Provider)
	require.Equal(t, "model-x", cfg.CodingAgent.Model)
	require.Equal(t, "warning", cfg.LogLevel)
	require.Nil(t, cfg.OverviewEnabled)
	require.Equal(t, "9090", cfg.Server.Port)
	require.Equal(t, "whsec", cfg.Server.GitHub.WebhookSecret)
	require.Equal(t, "12345", cfg.Server.GitHub.AppID)
	require.Equal(t, "-----BEGIN PRIVATE KEY-----\\nabc\\n-----END PRIVATE KEY-----", cfg.Server.GitHub.AppPrivateKey)
	require.Equal(t, "https://github.example.com/api/v3", cfg.Server.GitHub.APIBaseURL)
	require.Equal(t, "autogit", cfg.Server.GitHub.ReplyCommentTriggerName)
}

func TestLoadDoesNotOverrideExistingEnvWithDotEnv(t *testing.T) {
	t.Setenv("LLM_OPENAI_BASE_URL", "gemini")
	t.Setenv("LLM_OPENAI_MODEL", "from-env")
	t.Setenv("LLM_OPENAI_API_KEY", "env-key")
	t.Setenv("CODING_AGENT_NAME", "opencode")
	t.Setenv("CODING_AGENT_PROVIDER", "codingagent")
	t.Setenv("CODING_AGENT_MODEL", "model-y")
	t.Setenv("PORT", "7777")
	t.Setenv("LOG_LEVEL", "error")
	t.Setenv("OVERVIEW_ENABLED", "true")
	t.Setenv("GITHUB_WEBHOOK_SECRET", "set-secret")
	t.Setenv("GITHUB_APP_ID", "set-app-id")
	t.Setenv("GITHUB_APP_PRIVATE_KEY", "set-private-key")
	t.Setenv("GITHUB_API_BASE_URL", "https://set.example.com")
	t.Setenv("REPLYCOMMENT_TRIGGER_NAME", "set-trigger")
	t.Setenv("REVIEW_SUGGESTED_CHANGES", "true")

	tmp := t.TempDir()
	originalWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmp))
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})

	envPath := filepath.Join(tmp, ".env")
	require.NoError(t, os.WriteFile(envPath, []byte("LLM_OPENAI_BASE_URL=openai\nLLM_OPENAI_MODEL=from-dotenv\nLLM_OPENAI_API_KEY=dotenv-key\nCODING_AGENT_NAME=opencode\nCODING_AGENT_PROVIDER=dotenv-provider\nCODING_AGENT_MODEL=dotenv-model\nPORT=9090\nLOG_LEVEL=warning\nGITHUB_WEBHOOK_SECRET=dotenv-secret\nGITHUB_APP_ID=dotenv-app-id\nGITHUB_APP_PRIVATE_KEY=dotenv-private-key\nGITHUB_API_BASE_URL=https://dotenv.example.com\nREPLYCOMMENT_TRIGGER_NAME=dotenv-trigger\n"), 0o644))

	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, "gemini", cfg.OpenAI.BaseURL)
	require.Equal(t, "from-env", cfg.OpenAI.Model)
	require.Equal(t, "env-key", cfg.OpenAI.APIKey)
	require.Equal(t, "opencode", cfg.CodingAgent.Agent)
	require.Equal(t, "codingagent", cfg.CodingAgent.Provider)
	require.Equal(t, "model-y", cfg.CodingAgent.Model)
	require.Equal(t, "error", cfg.LogLevel)
	require.NotNil(t, cfg.OverviewEnabled)
	require.True(t, *cfg.OverviewEnabled)
	require.Equal(t, "7777", cfg.Server.Port)
	require.Equal(t, "set-secret", cfg.Server.GitHub.WebhookSecret)
	require.Equal(t, "set-app-id", cfg.Server.GitHub.AppID)
	require.Equal(t, "set-private-key", cfg.Server.GitHub.AppPrivateKey)
	require.Equal(t, "https://set.example.com", cfg.Server.GitHub.APIBaseURL)
	require.Equal(t, "set-trigger", cfg.Server.GitHub.ReplyCommentTriggerName)
	require.True(t, cfg.SuggestedChanges.Enabled)
	require.Equal(t, defaultSuggestedChangesMinSeverity, cfg.SuggestedChanges.MinSeverity)
	require.Equal(t, defaultSuggestedChangesMaxCandidates, cfg.SuggestedChanges.MaxCandidates)
	require.Equal(t, defaultSuggestedChangesMaxGroupSize, cfg.SuggestedChanges.MaxGroupSize)
	require.Equal(t, defaultSuggestedChangesMaxWorkers, cfg.SuggestedChanges.MaxWorkers)
	require.Equal(t, defaultSuggestedChangesGroupTimeoutMS, cfg.SuggestedChanges.GroupTimeoutMS)
	require.Equal(t, defaultSuggestedChangesGenerateTimeoutMS, cfg.SuggestedChanges.GenerateTimeoutMS)
}

func TestLoadReturnsErrorForInvalidDotEnv(t *testing.T) {
	unsetEnv(t, "LLM_OPENAI_BASE_URL")
	unsetEnv(t, "LLM_OPENAI_API_KEY")
	unsetEnv(t, "LLM_OPENAI_MODEL")
	unsetEnv(t, "CODING_AGENT_NAME")
	unsetEnv(t, "CODING_AGENT_PROVIDER")
	unsetEnv(t, "CODING_AGENT_MODEL")
	unsetEnv(t, "PORT")
	unsetEnv(t, "LOG_LEVEL")
	unsetEnv(t, "OVERVIEW_ENABLED")
	unsetEnv(t, "GITHUB_WEBHOOK_SECRET")
	unsetEnv(t, "GITHUB_APP_ID")
	unsetEnv(t, "GITHUB_APP_PRIVATE_KEY")
	unsetEnv(t, "GITHUB_API_BASE_URL")
	unsetEnv(t, "REPLYCOMMENT_TRIGGER_NAME")
	unsetEnv(t, "REVIEW_SUGGESTED_CHANGES")

	tmp := t.TempDir()
	originalWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmp))
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})

	envPath := filepath.Join(tmp, ".env")
	require.NoError(t, os.WriteFile(envPath, []byte("CODING_AGENT_NAME=\"unterminated\n"), 0o644))

	_, err = Load()
	require.Error(t, err)
}

func TestLoadParsesOverviewEnabledFalse(t *testing.T) {
	unsetEnv(t, "LLM_OPENAI_BASE_URL")
	unsetEnv(t, "LLM_OPENAI_API_KEY")
	unsetEnv(t, "LLM_OPENAI_MODEL")
	unsetEnv(t, "CODING_AGENT_NAME")
	unsetEnv(t, "CODING_AGENT_PROVIDER")
	unsetEnv(t, "CODING_AGENT_MODEL")
	unsetEnv(t, "PORT")
	unsetEnv(t, "LOG_LEVEL")
	unsetEnv(t, "GITHUB_WEBHOOK_SECRET")
	unsetEnv(t, "GITHUB_APP_ID")
	unsetEnv(t, "GITHUB_APP_PRIVATE_KEY")
	unsetEnv(t, "GITHUB_API_BASE_URL")
	unsetEnv(t, "REPLYCOMMENT_TRIGGER_NAME")
	unsetEnv(t, "REVIEW_SUGGESTED_CHANGES")
	t.Setenv("OVERVIEW_ENABLED", "false")

	tmp := t.TempDir()
	originalWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmp))
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})

	cfg, err := Load()
	require.NoError(t, err)
	require.NotNil(t, cfg.OverviewEnabled)
	require.False(t, *cfg.OverviewEnabled)
}

func TestLoadReturnsErrorForInvalidOverviewEnabled(t *testing.T) {
	unsetEnv(t, "LLM_OPENAI_BASE_URL")
	unsetEnv(t, "LLM_OPENAI_API_KEY")
	unsetEnv(t, "LLM_OPENAI_MODEL")
	unsetEnv(t, "CODING_AGENT_NAME")
	unsetEnv(t, "CODING_AGENT_PROVIDER")
	unsetEnv(t, "CODING_AGENT_MODEL")
	unsetEnv(t, "PORT")
	unsetEnv(t, "LOG_LEVEL")
	unsetEnv(t, "GITHUB_WEBHOOK_SECRET")
	unsetEnv(t, "GITHUB_APP_ID")
	unsetEnv(t, "GITHUB_APP_PRIVATE_KEY")
	unsetEnv(t, "GITHUB_API_BASE_URL")
	unsetEnv(t, "REVIEW_SUGGESTED_CHANGES")
	t.Setenv("OVERVIEW_ENABLED", "not-a-bool")

	tmp := t.TempDir()
	originalWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmp))
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})

	_, err = Load()
	require.Error(t, err)
	require.ErrorContains(t, err, "invalid OVERVIEW_ENABLED")
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
