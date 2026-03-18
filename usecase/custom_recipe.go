package usecase

import (
	"context"

	"github.com/bentos-lab/peer/domain"
	uccontracts "github.com/bentos-lab/peer/usecase/contracts"
)

// CustomRecipeLoader loads repo-scoped prompt customization.
type CustomRecipeLoader interface {
	Load(ctx context.Context, env uccontracts.CodeEnvironment, headRef string) (domain.CustomRecipe, error)
}
