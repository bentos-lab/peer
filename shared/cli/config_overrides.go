package cli

import "bentos-backend/config"

// ConfigOverrides captures explicit CLI overrides for config.Config.
type ConfigOverrides struct {
	LogLevel                *string
	OverviewEnabled         *bool
	SuggestedChangesEnabled *bool
	OpenAIBaseURL           *string
	OpenAIAPIKey            *string
	OpenAIModel             *string
	CodingAgentName         *string
	CodingAgentProvider     *string
	CodingAgentModel        *string
	ServerPort              *string
	GitHubWebhookSecret     *string
	GitHubAppID             *string
	GitHubAppPrivateKey     *string
	GitHubAPIBaseURL        *string
	GitHubReplyTriggerName  *string
}

// ApplyConfigOverrides applies explicit overrides to a base config.
func ApplyConfigOverrides(base config.Config, overrides ConfigOverrides) config.Config {
	if overrides.LogLevel != nil {
		base.LogLevel = *overrides.LogLevel
	}
	if overrides.OverviewEnabled != nil {
		base.OverviewEnabled = overrides.OverviewEnabled
	}
	if overrides.SuggestedChangesEnabled != nil {
		base.SuggestedChanges.Enabled = *overrides.SuggestedChangesEnabled
	}
	if overrides.OpenAIBaseURL != nil {
		base.OpenAI.BaseURL = *overrides.OpenAIBaseURL
	}
	if overrides.OpenAIAPIKey != nil {
		base.OpenAI.APIKey = *overrides.OpenAIAPIKey
	}
	if overrides.OpenAIModel != nil {
		base.OpenAI.Model = *overrides.OpenAIModel
	}
	if overrides.CodingAgentName != nil {
		base.CodingAgent.Agent = *overrides.CodingAgentName
	}
	if overrides.CodingAgentProvider != nil {
		base.CodingAgent.Provider = *overrides.CodingAgentProvider
	}
	if overrides.CodingAgentModel != nil {
		base.CodingAgent.Model = *overrides.CodingAgentModel
	}
	if overrides.ServerPort != nil {
		base.Server.Port = *overrides.ServerPort
	}
	if overrides.GitHubWebhookSecret != nil {
		base.Server.GitHub.WebhookSecret = *overrides.GitHubWebhookSecret
	}
	if overrides.GitHubAppID != nil {
		base.Server.GitHub.AppID = *overrides.GitHubAppID
	}
	if overrides.GitHubAppPrivateKey != nil {
		base.Server.GitHub.AppPrivateKey = *overrides.GitHubAppPrivateKey
	}
	if overrides.GitHubAPIBaseURL != nil {
		base.Server.GitHub.APIBaseURL = *overrides.GitHubAPIBaseURL
	}
	if overrides.GitHubReplyTriggerName != nil {
		base.Server.GitHub.ReplyCommentTriggerName = *overrides.GitHubReplyTriggerName
	}
	return base
}
