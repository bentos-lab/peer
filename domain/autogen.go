package domain

// AutogenChange captures one added content block for autogen output.
type AutogenChange struct {
	FilePath  string
	StartLine int
	EndLine   int
	Content   string
}

// AutogenSummary captures categorized additions produced by autogen.
type AutogenSummary struct {
	Tests    []string
	Docs     []string
	Comments []string
}
