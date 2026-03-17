package config

import (
	"errors"
	"io/fs"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config contains app runtime configuration.
type Config struct {
	LogLevel     string
	OpenAI       OpenAIConfig
	CodingAgent  CodingAgentConfig
	Server       ServerConfig
	Review       ReviewConfig
	Overview     OverviewConfig
	Autogen      AutogenConfig
	ReplyComment ReplyCommentConfig
}

// ReviewConfig contains review feature settings.
type ReviewConfig struct {
	Enabled                 bool
	SuggestedChangesEnabled bool
	Events                  []string
}

// OverviewConfig contains overview feature settings.
type OverviewConfig struct {
	Enabled               bool
	Events                []string
	IssueAlignmentEnabled bool
}

// AutogenConfig contains autogen feature settings.
type AutogenConfig struct {
	Enabled      bool
	Events       []string
	DocsEnabled  bool
	TestsEnabled bool
}

// ReplyCommentConfig contains replycomment feature settings.
type ReplyCommentConfig struct {
	Enabled     bool
	Events      []string
	Actions     []string
	TriggerName string
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
	Port          string
	MaxJobWorkers int
	GitHub        GitHubConfig
	GitLab        GitLabConfig
}

// GitHubConfig contains GitHub webhook/app integration settings.
type GitHubConfig struct {
	WebhookSecret string
	AppID         string
	AppPrivateKey string
	APIBaseURL    string
}

// GitLabConfig contains GitLab webhook/pat integration settings.
type GitLabConfig struct {
	Token         string
	WebhookSecret string
	APIBaseURL    string
	WebhookURL    string
	SyncInterval  int
	SyncStatePath string
}

// Load reads configuration from environment variables.
func Load() (Config, error) {
	if err := godotenv.Load(); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return Config{}, err
	}

	cfg := Config{
		LogLevel: envOrDefault("LOG_LEVEL", "info"),
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
			MaxJobWorkers: intEnvOrDefault("MAX_JOB_WORKERS", 3, 1),
			GitHub: GitHubConfig{
				WebhookSecret: os.Getenv("GITHUB_WEBHOOK_SECRET"),
				AppID:         os.Getenv("GITHUB_APP_ID"),
				AppPrivateKey: os.Getenv("GITHUB_APP_PRIVATE_KEY"),
				APIBaseURL:    envOrDefault("GITHUB_API_BASE_URL", "https://api.github.com"),
			},
			GitLab: GitLabConfig{
				Token:         os.Getenv("GITLAB_TOKEN"),
				WebhookSecret: os.Getenv("GITLAB_WEBHOOK_SECRET"),
				APIBaseURL:    resolveGitLabAPIBaseURL(),
				WebhookURL:    os.Getenv("GITLAB_WEBHOOK_URL"),
				SyncInterval:  intEnvOrDefault("GITLAB_SYNC_INTERVAL_MINUTES", 5, 1),
				SyncStatePath: envOrDefault("GITLAB_SYNC_STATE_PATH", "~/.autogit/gitlab_sync.json"),
			},
		},
		Review: ReviewConfig{
			Enabled:                 boolEnvOrDefault("REVIEW", true),
			SuggestedChangesEnabled: boolEnvOrDefault("REVIEW_SUGGESTED_CHANGES", false),
			Events:                  stringListEnvOrDefault("REVIEW_EVENTS", defaultReviewEvents()),
		},
		Overview: OverviewConfig{
			Enabled:               boolEnvOrDefault("OVERVIEW", true),
			Events:                stringListEnvOrDefault("OVERVIEW_EVENTS", defaultOverviewEvents()),
			IssueAlignmentEnabled: boolEnvOrDefault("OVERVIEW_ISSUE_ALIGNMENT", true),
		},
		Autogen: AutogenConfig{
			Enabled:      boolEnvOrDefault("AUTOGEN", false),
			Events:       stringListEnvOrDefault("AUTOGEN_EVENTS", defaultAutogenEvents()),
			DocsEnabled:  boolEnvOrDefault("AUTOGEN_DOCS", false),
			TestsEnabled: boolEnvOrDefault("AUTOGEN_TESTS", false),
		},
		ReplyComment: ReplyCommentConfig{
			Enabled:     boolEnvOrDefault("REPLYCOMMENT", true),
			Events:      stringListEnvOrDefault("REPLYCOMMENT_EVENTS", defaultReplyCommentEvents()),
			Actions:     stringListEnvOrDefault("REPLYCOMMENT_ACTIONS", defaultReplyCommentActions()),
			TriggerName: envOrDefault("REPLYCOMMENT_TRIGGER_NAME", "autogitbot"),
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

func stringListEnvOrDefault(key string, fallback []string) []string {
	rawValue, exists := os.LookupEnv(key)
	if !exists {
		return copyStringList(fallback)
	}
	trimmed := strings.TrimSpace(rawValue)
	if trimmed == "" {
		return []string{}
	}
	parts := strings.Split(trimmed, ",")
	normalized := normalizeStringList(parts)
	if normalized == nil {
		return []string{}
	}
	return normalized
}

func normalizeStringList(values []string) []string {
	if values == nil {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		lowered := strings.ToLower(trimmed)
		if _, exists := seen[lowered]; exists {
			continue
		}
		seen[lowered] = struct{}{}
		normalized = append(normalized, lowered)
	}
	if normalized == nil {
		return []string{}
	}
	return normalized
}

func copyStringList(values []string) []string {
	if values == nil {
		return nil
	}
	copied := make([]string, len(values))
	copy(copied, values)
	return copied
}

func defaultReviewEvents() []string {
	return []string{"opened", "synchronize", "reopened"}
}

func defaultOverviewEvents() []string {
	return []string{"opened"}
}

func defaultAutogenEvents() []string {
	return []string{"opened", "reopened", "synchronize"}
}

func defaultReplyCommentEvents() []string {
	return []string{"issue_comment", "pull_request_review_comment"}
}

func defaultReplyCommentActions() []string {
	return []string{"created"}
}

func intEnvOrDefault(key string, fallback int, min int) int {
	rawValue, exists := os.LookupEnv(key)
	if !exists {
		return fallback
	}

	parsedValue, err := strconv.Atoi(strings.TrimSpace(rawValue))
	if err != nil || parsedValue < min {
		return fallback
	}
	return parsedValue
}

func resolveGitLabAPIBaseURL() string {
	baseURL := strings.TrimSpace(os.Getenv("GITLAB_API_BASE_URL"))
	if baseURL != "" {
		return baseURL
	}
	host := strings.TrimSpace(os.Getenv("GITLAB_HOST"))
	if host == "" {
		host = "gitlab.com"
	}
	return "https://" + host + "/api/v4"
}
