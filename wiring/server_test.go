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
			BaseURL: "openai",
			APIKey:  "key",
			Model:   "gpt-4.1-mini",
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
			BaseURL: "openai",
			APIKey:  "key",
			Model:   "gpt-4.1-mini",
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
			BaseURL: "openai",
			APIKey:  "key",
			Model:   "gpt-4.1-mini",
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

func TestResolveServerOverviewEnabled(t *testing.T) {
	testCases := []struct {
		name     string
		input    config.Config
		expected bool
	}{
		{
			name:     "defaults to true when unset",
			input:    config.Config{},
			expected: true,
		},
		{
			name: "uses configured true",
			input: config.Config{
				OverviewEnabled: boolPointer(true),
			},
			expected: true,
		},
		{
			name: "uses configured false",
			input: config.Config{
				OverviewEnabled: boolPointer(false),
			},
			expected: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			actual := resolveServerOverviewEnabled(testCase.input)
			require.Equal(t, testCase.expected, actual)
		})
	}
}

func boolPointer(value bool) *bool {
	return &value
}

func mustGeneratePrivateKeyPEM(t *testing.T) string {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	encoded := x509.MarshalPKCS1PrivateKey(privateKey)
	block := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: encoded}
	return string(pem.EncodeToMemory(block))
}
