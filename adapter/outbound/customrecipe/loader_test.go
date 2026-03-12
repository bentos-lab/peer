package customrecipe

import (
	"context"
	"testing"

	"bentos-backend/domain"
	"bentos-backend/usecase"
	uccontracts "bentos-backend/usecase/contracts"

	"github.com/stretchr/testify/require"
)

type recipeTestEnvironment struct {
	files map[string]string
}

func (e *recipeTestEnvironment) SetupAgent(_ context.Context, _ domain.CodingAgentSetupOptions) (uccontracts.CodingAgent, error) {
	return nil, nil
}

func (e *recipeTestEnvironment) LoadChangedFiles(_ context.Context, _ domain.CodeEnvironmentLoadOptions) ([]domain.ChangedFile, error) {
	return nil, nil
}

func (e *recipeTestEnvironment) ReadFile(_ context.Context, path string, _ string) (string, bool, error) {
	content, ok := e.files[path]
	return content, ok, nil
}

func (e *recipeTestEnvironment) PushChanges(_ context.Context, _ domain.CodeEnvironmentPushOptions) (domain.CodeEnvironmentPushResult, error) {
	return domain.CodeEnvironmentPushResult{}, nil
}

func (e *recipeTestEnvironment) Cleanup(_ context.Context) error {
	return nil
}

type recipeTestSanitizer struct {
	status domain.PromptSafetyStatusEnum
}

func (s *recipeTestSanitizer) Sanitize(_ context.Context, prompt string) (usecase.SanitizedPrompt, error) {
	return usecase.SanitizedPrompt{
		Status:          s.status,
		SanitizedPrompt: prompt,
	}, nil
}

func TestLoaderReturnsEmptyRecipeWhenConfigMissing(t *testing.T) {
	loader, err := NewLoader(&recipeTestSanitizer{status: domain.PromptSafetyStatusOK}, &recipeTestSanitizer{status: domain.PromptSafetyStatusOK}, nil)
	require.NoError(t, err)

	recipe, err := loader.Load(context.Background(), &recipeTestEnvironment{files: map[string]string{}}, "HEAD")
	require.NoError(t, err)
	require.Equal(t, domain.CustomRecipe{}, recipe)
}

func TestLoaderReadsAndSanitizesRecipeFiles(t *testing.T) {
	loader, err := NewLoader(&recipeTestSanitizer{status: domain.PromptSafetyStatusOK}, &recipeTestSanitizer{status: domain.PromptSafetyStatusOK}, nil)
	require.NoError(t, err)

	env := &recipeTestEnvironment{files: map[string]string{
		".autogit/config.toml": `
[review]
ruleset = "rules.md"
suggestions = true

[review.overview]
enabled = false
extra_guidance = "overview.md"

[autoreply]
extra_guidance = "reply.md"

[autogen]
extra_guidance = "autogen.md"
`,
		".autogit/rules.md":    "rules",
		".autogit/overview.md": "overview",
		".autogit/reply.md":    "reply",
		".autogit/autogen.md":  "autogen",
	}}

	recipe, err := loader.Load(context.Background(), env, "HEAD")
	require.NoError(t, err)
	require.Equal(t, "rules", recipe.ReviewRuleset)
	require.Equal(t, "overview", recipe.ReviewOverviewGuidance)
	require.Equal(t, "reply", recipe.AutoreplyGuidance)
	require.Equal(t, "autogen", recipe.AutogenGuidance)
	require.NotNil(t, recipe.ReviewSuggestions)
	require.True(t, *recipe.ReviewSuggestions)
	require.NotNil(t, recipe.ReviewOverview)
	require.False(t, *recipe.ReviewOverview)
}

func TestLoaderIgnoresInvalidRecipePath(t *testing.T) {
	loader, err := NewLoader(&recipeTestSanitizer{status: domain.PromptSafetyStatusOK}, &recipeTestSanitizer{status: domain.PromptSafetyStatusOK}, nil)
	require.NoError(t, err)

	env := &recipeTestEnvironment{files: map[string]string{
		".autogit/config.toml": `
[review]
ruleset = "../rules.md"
`,
	}}

	recipe, err := loader.Load(context.Background(), env, "HEAD")
	require.NoError(t, err)
	require.Empty(t, recipe.ReviewRuleset)
}

func TestLoaderRecordsMissingRecipePaths(t *testing.T) {
	loader, err := NewLoader(&recipeTestSanitizer{status: domain.PromptSafetyStatusOK}, &recipeTestSanitizer{status: domain.PromptSafetyStatusOK}, nil)
	require.NoError(t, err)

	env := &recipeTestEnvironment{files: map[string]string{
		".autogit/config.toml": `
[review]
ruleset = "rules.md"

[review.overview]
enabled = true
extra_guidance = "overview.md"
`,
	}}

	recipe, err := loader.Load(context.Background(), env, "HEAD")
	require.NoError(t, err)
	require.ElementsMatch(t, []string{".autogit/rules.md", ".autogit/overview.md"}, recipe.MissingPaths)
}
