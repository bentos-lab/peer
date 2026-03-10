package domain

// ChangedFile represents one changed file content used for review.
type ChangedFile struct {
	Path        string
	Content     string
	OldContent  string
	DiffSnippet string
}
