package usecase

import (
	"context"

	"bentos-backend/domain"
	uccontracts "bentos-backend/usecase/contracts"
)

// CustomRecipeLoader loads repo-scoped prompt customization.
type CustomRecipeLoader interface {
	Load(ctx context.Context, env uccontracts.CodeEnvironment, headRef string) (domain.CustomRecipe, error)
}
