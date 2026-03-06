package domain

// OverviewCategoryEnum identifies one fixed high-level change category.
type OverviewCategoryEnum string

const (
	// OverviewCategoryLogicUpdates groups behavior and logic changes.
	OverviewCategoryLogicUpdates OverviewCategoryEnum = "Logic Updates"
	// OverviewCategoryRefactoring groups structural/code quality changes.
	OverviewCategoryRefactoring OverviewCategoryEnum = "Refactoring"
	// OverviewCategorySecurityFixes groups security-relevant fixes.
	OverviewCategorySecurityFixes OverviewCategoryEnum = "Security Fixes"
	// OverviewCategoryTestChanges groups test suite updates.
	OverviewCategoryTestChanges OverviewCategoryEnum = "Test Changes"
	// OverviewCategoryDocumentation groups docs updates.
	OverviewCategoryDocumentation OverviewCategoryEnum = "Documentation"
	// OverviewCategoryInfrastructureConfig groups infra/config changes.
	OverviewCategoryInfrastructureConfig OverviewCategoryEnum = "Infrastructure/Config"
)

// OverviewCategoryItem summarizes one category in the overview output.
type OverviewCategoryItem struct {
	Category OverviewCategoryEnum
	Summary  string
}

// OverviewWalkthrough describes one grouped "story" slice of the change.
type OverviewWalkthrough struct {
	GroupName string
	Files     []string
	Summary   string
}
