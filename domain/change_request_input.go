package domain

// ChangeRequestTarget represents where change-request outputs should be published.
type ChangeRequestTarget struct {
	Repository          string
	ChangeRequestNumber int
}

// ChangeRequestInput is the shared input for review and overview generation.
type ChangeRequestInput struct {
	Target      ChangeRequestTarget
	RepoURL     string
	Base        string
	Head        string
	Title       string
	Description string
	Language    string
	Metadata    map[string]string
}
