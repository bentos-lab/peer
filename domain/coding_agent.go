package domain

// CodingAgentSetupOptions contains setup inputs for creating a coding agent.
type CodingAgentSetupOptions struct {
	Agent string
	// Ref is the target head ref to checkout before SetupAgent returns the coding agent.
	Ref string
}

// CodeEnvironmentLoadOptions contains inputs for loading changed files from a workspace.
type CodeEnvironmentLoadOptions struct {
	Base string
	Head string
}

// CodeEnvironmentInitOptions contains request-scoped context for environment initialization.
type CodeEnvironmentInitOptions struct {
	RepoURL string
	// UseCwd uses the current working directory directly for local repositories.
	UseCwd bool
}

// CodeEnvironmentPushOptions contains inputs for pushing code changes.
type CodeEnvironmentPushOptions struct {
	TargetBranch  string
	CommitMessage string
	RemoteName    string
}

// CodeEnvironmentPushResult captures push results for a code environment.
type CodeEnvironmentPushResult struct {
	Pushed bool
}

// CodeEnvironmentCommitOptions contains inputs for committing code changes.
type CodeEnvironmentCommitOptions struct {
	CommitMessage string
	StageAll      bool
}

// CodeEnvironmentCommitResult captures commit results for a code environment.
type CodeEnvironmentCommitResult struct {
	Committed bool
}

// CodingAgentRunOptions contains run-time inputs for one coding task execution.
type CodingAgentRunOptions struct {
	Provider string
	Model    string
	// SessionID continues a prior agent session when available.
	SessionID string
}

// CodingAgentRunResult is the output from one coding task execution.
type CodingAgentRunResult struct {
	Text      string
	SessionID string
}
