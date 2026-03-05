package domain

// ChangedFile represents one changed file content used for review.
type ChangedFile struct {
	Path        string
	Content     string
	OldContent  string
	DiffSnippet string
}

// ReviewTarget represents where review comments should be published.
type ReviewTarget struct {
	Repository          string
	ChangeRequestNumber int
}

// ReviewInput is the complete input for the review engine.
type ReviewInput struct {
	Target        ReviewTarget
	Title         string
	Description   string
	ChangedFiles  []ChangedFile
	Language      string
	Metadata      map[string]string
	SourceContext string
}
