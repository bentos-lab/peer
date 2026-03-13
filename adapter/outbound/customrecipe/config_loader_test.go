package customrecipe

import (
	"context"
	"testing"

	"bentos-backend/domain"
	uccontracts "bentos-backend/usecase/contracts"
	"github.com/stretchr/testify/require"
)

type recipeTestFactory struct {
	env *recipeTestEnvironment
}

func (f *recipeTestFactory) New(_ context.Context, _ domain.CodeEnvironmentInitOptions) (uccontracts.CodeEnvironment, error) {
	return f.env, nil
}

func TestConfigLoaderReturnsEmptyWhenConfigMissing(t *testing.T) {
	loader, err := NewConfigLoader(&recipeTestFactory{env: &recipeTestEnvironment{files: map[string]string{}}}, nil)
	require.NoError(t, err)

	recipe, err := loader.Load(context.Background(), "https://example.com/repo.git", "HEAD")
	require.NoError(t, err)
	require.Equal(t, domain.CustomRecipe{}, recipe)
}

func TestConfigLoaderReadsEnabledFlags(t *testing.T) {
	loader, err := NewConfigLoader(&recipeTestFactory{env: &recipeTestEnvironment{files: map[string]string{
		".autogit/config.toml": `
[review]
enabled = false

[overview]
enabled = true

[overview.issue_alignment]
enabled = false

[autoreply]
enabled = true

[autogen]
enabled = false
`,
	}}}, nil)
	require.NoError(t, err)

	recipe, err := loader.Load(context.Background(), "https://example.com/repo.git", "HEAD")
	require.NoError(t, err)
	require.NotNil(t, recipe.ReviewEnabled)
	require.False(t, *recipe.ReviewEnabled)
	require.NotNil(t, recipe.OverviewEnabled)
	require.True(t, *recipe.OverviewEnabled)
	require.NotNil(t, recipe.OverviewIssueAlignmentEnabled)
	require.False(t, *recipe.OverviewIssueAlignmentEnabled)
	require.NotNil(t, recipe.AutoreplyEnabled)
	require.True(t, *recipe.AutoreplyEnabled)
	require.NotNil(t, recipe.AutogenEnabled)
	require.False(t, *recipe.AutogenEnabled)
}
