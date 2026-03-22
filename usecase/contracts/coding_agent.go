package contracts

import (
	"context"

	"github.com/bentos-lab/peer/domain"
)

// CodingAgent runs coding tasks in a prepared environment.
type CodingAgent interface {
	Run(ctx context.Context, task string, opts domain.CodingAgentRunOptions) (domain.CodingAgentRunResult, error)
}

// CodeEnvironment provides repository-scoped code operations for review/overview.
type CodeEnvironment interface {
	// SetupAgent prepares a coding agent against the target repository context.
	SetupAgent(ctx context.Context, opts domain.CodingAgentSetupOptions) (CodingAgent, error)
	// ResolveBaseHead resolves base/head refs into concrete commit refs when needed.
	ResolveBaseHead(ctx context.Context, base string, head string) (string, string, error)
	// LoadChangedFiles loads changed files for the selected comparison mode.
	LoadChangedFiles(ctx context.Context, opts domain.CodeEnvironmentLoadOptions) ([]domain.ChangedFile, error)
	// EnsureDiffContentAvailable validates that diff content is present for the requested comparison.
	EnsureDiffContentAvailable(ctx context.Context, opts domain.CodeEnvironmentLoadOptions) error
	// ReadFile reads a repository-relative file at the provided ref.
	ReadFile(ctx context.Context, path string, ref string) (string, bool, error)
	// CommitChanges commits changes in the code environment.
	CommitChanges(ctx context.Context, opts domain.CodeEnvironmentCommitOptions) (domain.CodeEnvironmentCommitResult, error)
	// PushChanges commits and pushes changes with the provided options.
	PushChanges(ctx context.Context, opts domain.CodeEnvironmentPushOptions) (domain.CodeEnvironmentPushResult, error)
	// Cleanup releases any resources allocated for the code environment.
	Cleanup(ctx context.Context) error
}

// CodeEnvironmentFactory creates request-scoped code environments.
type CodeEnvironmentFactory interface {
	// New creates a code environment for one usecase execution context.
	New(ctx context.Context, opts domain.CodeEnvironmentInitOptions) (CodeEnvironment, error)
}
