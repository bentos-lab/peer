package domain

// CustomRecipe captures repo-scoped prompt customization settings.
type CustomRecipe struct {
	ReviewRuleset          string
	ReviewSuggestions      *bool
	ReviewOverview         *bool
	ReviewOverviewGuidance string
	AutoreplyGuidance      string
	AutogenGuidance        string
	MissingPaths           []string
}
