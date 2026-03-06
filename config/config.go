package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config contains app runtime configuration.
type Config struct {
	LogLevel        string
	OverviewEnabled *bool
	OpenAI          OpenAIConfig
	Server          ServerConfig
}

// OpenAIConfig contains OpenAI-compatible provider settings.
type OpenAIConfig struct {
	BaseURL string
	APIKey  string
	Model   string
}

// ServerConfig contains HTTP server-specific settings.
type ServerConfig struct {
	Port   string
	GitHub GitHubConfig
}

// GitHubConfig contains GitHub webhook/app integration settings.
type GitHubConfig struct {
	WebhookSecret string
	AppID         string
	AppPrivateKey string
	APIBaseURL    string
}

// Load reads configuration from environment variables.
func Load() (Config, error) {
	if err := godotenv.Load(); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return Config{}, err
	}

	overviewEnabled, err := optionalBoolEnv("OVERVIEW_ENABLED")
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		LogLevel:        envOrDefault("LOG_LEVEL", "info"),
		OverviewEnabled: overviewEnabled,
		OpenAI: OpenAIConfig{
			BaseURL: envOrDefault("OPENAI_BASE_URL", "gemini"),
			APIKey:  os.Getenv("OPENAI_API_KEY"),
			Model:   envOrDefault("OPENAI_MODEL", "gemini-2.5-flash-lite"),
		},
		Server: ServerConfig{
			Port: envOrDefault("PORT", "8080"),
			GitHub: GitHubConfig{
				WebhookSecret: os.Getenv("GITHUB_WEBHOOK_SECRET"),
				AppID:         os.Getenv("GITHUB_APP_ID"),
				AppPrivateKey: os.Getenv("GITHUB_APP_PRIVATE_KEY"),
				APIBaseURL:    envOrDefault("GITHUB_API_BASE_URL", "https://api.github.com"),
			},
		},
	}
	return cfg, nil
}

func envOrDefault(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func optionalBoolEnv(key string) (*bool, error) {
	rawValue, exists := os.LookupEnv(key)
	if !exists {
		return nil, nil
	}

	parsedValue, err := strconv.ParseBool(strings.TrimSpace(rawValue))
	if err != nil {
		return nil, fmt.Errorf("invalid %s: %w", key, err)
	}

	return &parsedValue, nil
}
