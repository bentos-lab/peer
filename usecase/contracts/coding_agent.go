package contracts

import (
	"context"

	"bentos-backend/domain"
)

// CodingAgent runs coding tasks in a prepared environment.
type CodingAgent interface {
	Run(ctx context.Context, task string, opts domain.CodingAgentRunOptions) (domain.CodingAgentRunResult, error)
}

// CodingAgentEnvironment prepares coding agents for repository-scoped work.
type CodingAgentEnvironment interface {
	SetupAgent(ctx context.Context, opts domain.CodingAgentSetupOptions) (CodingAgent, error)
}
