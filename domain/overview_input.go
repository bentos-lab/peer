package domain

// OverviewTarget represents where overview output should be published.
type OverviewTarget struct {
	Repository          string
	ChangeRequestNumber int
}

// OverviewInput is the complete input for overview generation.
type OverviewInput struct {
	Target      OverviewTarget
	RepoURL     string
	Base        string
	Head        string
	Title       string
	Description string
	Language    string
	Metadata    map[string]string
}
