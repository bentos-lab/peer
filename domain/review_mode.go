package domain

// ReviewInputProvider identifies the source provider for review input.
type ReviewInputProvider string

const (
	// ReviewInputProviderLocal loads review input from the local repository.
	ReviewInputProviderLocal ReviewInputProvider = "local"
	// ReviewInputProviderGitHub loads review input from a GitHub pull request.
	ReviewInputProviderGitHub ReviewInputProvider = "github"
)

// ReviewPublishType identifies the delivery type for review results.
type ReviewPublishType string

const (
	// ReviewPublishTypePrint prints review results to standard output.
	ReviewPublishTypePrint ReviewPublishType = "print"
	// ReviewPublishTypeComment publishes review results as comments.
	ReviewPublishTypeComment ReviewPublishType = "comment"
)
