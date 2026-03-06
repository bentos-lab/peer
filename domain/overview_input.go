package domain

// OverviewTarget represents where overview output should be published.
type OverviewTarget struct {
	Repository          string
	ChangeRequestNumber int
}

// OverviewInput is the complete input for overview generation.
type OverviewInput struct {
	Target        OverviewTarget
	Title         string
	Description   string
	ChangedFiles  []ChangedFile
	Language      string
	Metadata      map[string]string
	SourceContext string
}
