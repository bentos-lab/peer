package domain

// CodingAgentSetupOptions contains setup inputs for creating a coding agent.
type CodingAgentSetupOptions struct {
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
