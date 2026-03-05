package config

import (
	"errors"
	"io/fs"
	"os"

	"github.com/joho/godotenv"
)

// Config contains app runtime configuration.
type Config struct {
	Port          string
	LogLevel      string
	OpenAIBaseURL string
	OpenAIAPIKey  string
	OpenAIModel   string
}

// Load reads configuration from environment variables.
func Load() (Config, error) {
	if err := godotenv.Load(); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return Config{}, err
	}

	cfg := Config{
		Port:          envOrDefault("PORT", "8080"),
		LogLevel:      envOrDefault("LOG_LEVEL", "info"),
		OpenAIBaseURL: envOrDefault("OPENAI_BASE_URL", "gemini"),
		OpenAIAPIKey:  os.Getenv("OPENAI_API_KEY"),
		OpenAIModel:   envOrDefault("OPENAI_MODEL", "gemini-2.5-flash-lite"),
	}
	return cfg, nil
}

func envOrDefault(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
