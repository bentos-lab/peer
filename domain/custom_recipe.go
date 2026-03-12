package domain

// CustomRecipe captures repo-scoped prompt customization settings.
type CustomRecipe struct {
	ReviewRuleset     string
	ReviewEnabled     *bool
	ReviewSuggestions *bool
	OverviewEnabled   *bool
	OverviewGuidance  string
	AutoreplyGuidance string
	AutogenGuidance   string
	MissingPaths      []string
}
