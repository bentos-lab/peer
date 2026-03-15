package domain

// CustomRecipe captures repo-scoped prompt customization settings.
type CustomRecipe struct {
	ReviewRuleset                  string
	ReviewEnabled                  *bool
	ReviewSuggestions              *bool
	ReviewEvents                   []string
	OverviewEnabled                *bool
	OverviewIssueAlignmentEnabled  *bool
	OverviewGuidance               string
	OverviewEvents                 []string
	OverviewIssueAlignmentGuidance string
	AutoreplyEnabled               *bool
	AutoreplyGuidance              string
	AutoreplyEvents                []string
	AutoreplyActions               []string
	AutogenEnabled                 *bool
	AutogenGuidance                string
	AutogenEvents                  []string
	AutogenDocs                    *bool
	AutogenTests                   *bool
	MissingPaths                   []string
}
