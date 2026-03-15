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
	LogLevel         string
	OverviewEnabled  *bool
	SuggestedChanges SuggestedChangesConfig
	OpenAI           OpenAIConfig
	CodingAgent      CodingAgentConfig
	Server           ServerConfig
}

const (
	defaultSuggestedChangesMinSeverity       = "MAJOR"
	defaultSuggestedChangesMaxCandidates     = 50
	defaultSuggestedChangesMaxGroupSize      = 5
	defaultSuggestedChangesMaxWorkers        = 3
	defaultSuggestedChangesGroupTimeoutMS    = 20000
	defaultSuggestedChangesGenerateTimeoutMS = 30000
)

// SuggestedChangesConfig contains suggested changes pipeline settings.
type SuggestedChangesConfig struct {
	Enabled           bool
	MinSeverity       string
	MaxCandidates     int
	MaxGroupSize      int
	MaxWorkers        int
	GroupTimeoutMS    int
	GenerateTimeoutMS int
}

// OpenAIConfig contains OpenAI-compatible provider settings.
type OpenAIConfig struct {
	BaseURL string
	APIKey  string
	Model   string
}

// CodingAgentConfig contains coding-agent runtime settings.
type CodingAgentConfig struct {
	Agent    string
	Provider string
	Model    string
}

// ServerConfig contains HTTP server-specific settings.
type ServerConfig struct {
	Port   string
	GitHub GitHubConfig
}

// GitHubConfig contains GitHub webhook/app integration settings.
type GitHubConfig struct {
	WebhookSecret           string
	AppID                   string
	AppPrivateKey           string
	APIBaseURL              string
	ReplyCommentTriggerName string
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
		SuggestedChanges: SuggestedChangesConfig{
			Enabled:           boolEnvOrDefault("REVIEW_SUGGESTED_CHANGES", false),
			MinSeverity:       defaultSuggestedChangesMinSeverity,
			MaxCandidates:     defaultSuggestedChangesMaxCandidates,
			MaxGroupSize:      defaultSuggestedChangesMaxGroupSize,
			MaxWorkers:        defaultSuggestedChangesMaxWorkers,
			GroupTimeoutMS:    defaultSuggestedChangesGroupTimeoutMS,
			GenerateTimeoutMS: defaultSuggestedChangesGenerateTimeoutMS,
		},
		OpenAI: OpenAIConfig{
			BaseURL: envOrDefault("LLM_OPENAI_BASE_URL", ""),
			APIKey:  os.Getenv("LLM_OPENAI_API_KEY"),
			Model:   envOrDefault("LLM_OPENAI_MODEL", ""),
		},
		CodingAgent: CodingAgentConfig{
			Agent:    envOrDefault("CODING_AGENT_NAME", "opencode"),
			Provider: os.Getenv("CODING_AGENT_PROVIDER"),
			Model:    os.Getenv("CODING_AGENT_MODEL"),
		},
		Server: ServerConfig{
			Port: func() string {
				port := envOrDefault("PORT", "8080")
				if p, err := strconv.Atoi(port); err == nil && p > 0 && p <= 65535 {
					return port
				}
				return "8080"
			}(),
			GitHub: GitHubConfig{
				WebhookSecret:           os.Getenv("GITHUB_WEBHOOK_SECRET"),
				AppID:                   os.Getenv("GITHUB_APP_ID"),
				AppPrivateKey:           os.Getenv("GITHUB_APP_PRIVATE_KEY"),
				APIBaseURL:              envOrDefault("GITHUB_API_BASE_URL", "https://api.github.com"),
				ReplyCommentTriggerName: envOrDefault("REPLYCOMMENT_TRIGGER_NAME", "autogitbot"),
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

func boolEnvOrDefault(key string, fallback bool) bool {
	rawValue, exists := os.LookupEnv(key)
	if !exists {
		return fallback
	}

	parsedValue, err := strconv.ParseBool(strings.TrimSpace(rawValue))
	if err != nil {
		return fallback
	}
	return parsedValue
}
