package domain

// CustomRecipe captures repo-scoped prompt customization settings.
type CustomRecipe struct {
	ReviewRuleset                 string
	ReviewEnabled                 *bool
	ReviewSuggestions             *bool
	OverviewEnabled               *bool
	OverviewIssueAlignmentEnabled *bool
	OverviewGuidance              string
	AutoreplyEnabled              *bool
	AutoreplyGuidance             string
	AutogenEnabled                *bool
	AutogenGuidance               string
	MissingPaths                  []string
}
