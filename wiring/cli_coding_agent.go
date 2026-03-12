package wiring

import (
	"strings"

	"bentos-backend/config"
)

// ResolveCLICodingAgentConfig applies CLI overrides to the coding agent config.
func ResolveCLICodingAgentConfig(cfg config.Config, opts CLILLMOptions) config.CodingAgentConfig {
	resolved := cfg.CodingAgent
	if opts.CodeAgentSet {
		resolved.Agent = strings.TrimSpace(opts.CodeAgent)
	}
	if opts.CodeAgentProviderSet {
		resolved.Provider = strings.TrimSpace(opts.CodeAgentProvider)
	}
	if opts.CodeAgentModelSet {
		resolved.Model = strings.TrimSpace(opts.CodeAgentModel)
	}
	return resolved
}
