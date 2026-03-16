package wiring

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"

	"bentos-backend/config"
	"github.com/stretchr/testify/require"
)

func TestBuildGitHubHandlerRejectsMissingWebhookSecret(t *testing.T) {
	_, err := BuildGitHubHandler(config.Config{
		LogLevel: "info",
		OpenAI: config.OpenAIConfig{
			BaseURL: "",
		},
		CodingAgent: config.CodingAgentConfig{
			Agent: "opencode",
		},
		Server: config.ServerConfig{
			GitHub: config.GitHubConfig{
				AppID:         "12345",
				AppPrivateKey: mustGeneratePrivateKeyPEM(t),
				APIBaseURL:    "https://api.github.com",
			},
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "github webhook secret is required")
}

func TestBuildGitHubHandlerRejectsMissingAppID(t *testing.T) {
	_, err := BuildGitHubHandler(config.Config{
		LogLevel: "info",
		OpenAI: config.OpenAIConfig{
			BaseURL: "",
		},
		CodingAgent: config.CodingAgentConfig{
			Agent: "opencode",
		},
		Server: config.ServerConfig{
			GitHub: config.GitHubConfig{
				WebhookSecret: "secret",
				AppPrivateKey: mustGeneratePrivateKeyPEM(t),
				APIBaseURL:    "https://api.github.com",
			},
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "github app ID is required")
}

func TestBuildGitHubHandlerBuildsWithRequiredConfig(t *testing.T) {
	handler, err := BuildGitHubHandler(config.Config{
		LogLevel: "info",
		OpenAI: config.OpenAIConfig{
			BaseURL: "",
		},
		CodingAgent: config.CodingAgentConfig{
			Agent: "opencode",
		},
		Server: config.ServerConfig{
			GitHub: config.GitHubConfig{
				WebhookSecret: "secret",
				AppID:         "12345",
				AppPrivateKey: mustGeneratePrivateKeyPEM(t),
				APIBaseURL:    "https://api.github.com",
			},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, handler)
}

func mustGeneratePrivateKeyPEM(t *testing.T) string {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	encoded := x509.MarshalPKCS1PrivateKey(privateKey)
	block := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: encoded}
	return string(pem.EncodeToMemory(block))
}
