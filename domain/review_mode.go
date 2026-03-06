package domain

// ChangeRequestInputProvider identifies the source provider for change snapshot input.
type ChangeRequestInputProvider string

const (
	// ChangeRequestInputProviderLocal loads input from the local repository.
	ChangeRequestInputProviderLocal ChangeRequestInputProvider = "local"
	// ChangeRequestInputProviderGitHub loads input from a GitHub pull request.
	ChangeRequestInputProviderGitHub ChangeRequestInputProvider = "github"
)

// ChangeRequestPublishType identifies the delivery type for review results.
type ChangeRequestPublishType string

const (
	// ChangeRequestPublishTypePrint prints review results to standard output.
	ChangeRequestPublishTypePrint ChangeRequestPublishType = "print"
	// ChangeRequestPublishTypeComment publishes review results as comments.
	ChangeRequestPublishTypeComment ChangeRequestPublishType = "comment"
)
