package domain

// ChangeRequestContext identifies one pull/merge request context.
type ChangeRequestContext struct {
	Repository          string
	RepoURL             string
	ChangeRequestNumber int
	Title               string
	Description         string
	Base                string
	Head                string
	Metadata            map[string]string
}

// ChangeSnapshot is the neutral changed-content input shared by features.
type ChangeSnapshot struct {
	Context       ChangeRequestContext
	ChangedFiles  []ChangedFile
	Language      string
	SourceContext string
}
