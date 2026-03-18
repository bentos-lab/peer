package llm

import (
	"fmt"
	"strings"
)

const (
	// ShortcutGemini identifies Gemini OpenAI-compatible base URL.
	ShortcutGemini = "gemini"
	// ShortcutOpenAI identifies OpenAI base URL.
	ShortcutOpenAI = "openai"
	// ShortcutAnthropic identifies Anthropic OpenAI-compatible base URL.
	ShortcutAnthropic = "anthropic"
)

var shortcutConfigs = map[string]struct {
	baseURL string
	model   string
}{
	ShortcutGemini: {
		baseURL: "https://generativelanguage.googleapis.com/v1beta/openai",
		model:   "gemini-2.5-flash-lite",
	},
	ShortcutOpenAI: {
		baseURL: "https://api.openai.com/v1",
		model:   "gpt-4.1-mini",
	},
	ShortcutAnthropic: {
		baseURL: "https://api.anthropic.com/v1",
		model:   "claude-3-5-haiku-latest",
	},
}

// ResolveBaseURLAndModel resolves shortcut/full URL and model precedence for CLI.
func ResolveBaseURLAndModel(baseURLInput, configModel, flagModel string) (string, string, bool, error) {
	baseURLInput = strings.TrimSpace(baseURLInput)
	configModel = strings.TrimSpace(configModel)
	flagModel = strings.TrimSpace(flagModel)
	if baseURLInput == "" {
		return "", "", false, fmt.Errorf("openai base URL is required")
	}

	shortcut := strings.ToLower(baseURLInput)
	if cfg, ok := shortcutConfigs[shortcut]; ok {
		if flagModel != "" {
			return cfg.baseURL, flagModel, true, nil
		}
		return cfg.baseURL, cfg.model, true, nil
	}

	if flagModel != "" {
		return baseURLInput, flagModel, false, nil
	}
	if configModel != "" {
		return baseURLInput, configModel, false, nil
	}

	return "", "", false, fmt.Errorf("openai model is required when using full base URL")
}
