package codingagent

import (
	"fmt"
	"strings"

	"github.com/bentos-lab/peer/domain"
)

func validateOverviewCategories(categories []domain.OverviewCategoryItem) error {
	allowed := map[domain.OverviewCategoryEnum]struct{}{
		domain.OverviewCategoryLogicUpdates:         {},
		domain.OverviewCategoryRefactoring:          {},
		domain.OverviewCategorySecurityFixes:        {},
		domain.OverviewCategoryTestChanges:          {},
		domain.OverviewCategoryDocumentation:        {},
		domain.OverviewCategoryInfrastructureConfig: {},
	}
	for _, category := range categories {
		if _, ok := allowed[category.Category]; !ok {
			return fmt.Errorf("invalid overview category: %q", category.Category)
		}
		if strings.TrimSpace(category.Summary) == "" {
			return fmt.Errorf("invalid overview category summary for %q", category.Category)
		}
	}
	return nil
}
