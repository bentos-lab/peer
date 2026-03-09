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
}

// CodingAgentRunOptions contains run-time inputs for one coding task execution.
type CodingAgentRunOptions struct {
	Provider string
	Model    string
}

// CodingAgentRunResult is the output from one coding task execution.
type CodingAgentRunResult struct {
	Text string
}
