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
	ReplyCommentEnabled            *bool
	ReplyCommentGuidance           string
	ReplyCommentEvents             []string
	ReplyCommentActions            []string
	AutogenEnabled                 *bool
	AutogenGuidance                string
	AutogenEvents                  []string
	AutogenDocs                    *bool
	AutogenTests                   *bool
	MissingPaths                   []string
}
