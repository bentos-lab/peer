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
events = ["opened", "reopened"]

[overview]
enabled = true
events = ["opened"]

[overview.issue_alignment]
enabled = false

[autoreply]
enabled = true
events = ["issue_comment"]
actions = ["created"]

[autogen]
enabled = false
events = ["opened"]
docs = true
tests = false
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
	require.Equal(t, []string{"opened", "reopened"}, recipe.ReviewEvents)
	require.Equal(t, []string{"opened"}, recipe.OverviewEvents)
	require.Equal(t, []string{"issue_comment"}, recipe.AutoreplyEvents)
	require.Equal(t, []string{"created"}, recipe.AutoreplyActions)
	require.Equal(t, []string{"opened"}, recipe.AutogenEvents)
	require.NotNil(t, recipe.AutogenDocs)
	require.True(t, *recipe.AutogenDocs)
	require.NotNil(t, recipe.AutogenTests)
	require.False(t, *recipe.AutogenTests)
}
