package customrecipe

import (
	"context"
	"testing"

	"github.com/bentos-lab/peer/domain"
	"github.com/bentos-lab/peer/usecase"
	uccontracts "github.com/bentos-lab/peer/usecase/contracts"

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

func (e *recipeTestEnvironment) CommitChanges(_ context.Context, _ domain.CodeEnvironmentCommitOptions) (domain.CodeEnvironmentCommitResult, error) {
	return domain.CodeEnvironmentCommitResult{}, nil
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
		".peer/config.toml": `
[review]
enabled = false
ruleset = "rules.md"
suggestions = true
events = ["opened", "synchronize"]

[overview]
enabled = false
extra_guidance = "overview.md"
events = ["opened"]

[overview.issue_alignment]
enabled = false
extra_guidance = "issue_alignment.md"

[replycomment]
enabled = true
extra_guidance = "reply.md"
events = ["issue_comment", "pull_request_review_comment"]
actions = ["created", "edited"]

[autogen]
enabled = true
extra_guidance = "autogen.md"
events = ["opened"]
docs = true
tests = false
`,
		".peer/rules.md":           "rules",
		".peer/overview.md":        "overview",
		".peer/issue_alignment.md": "alignment",
		".peer/reply.md":           "reply",
		".peer/autogen.md":         "autogen",
	}}

	recipe, err := loader.Load(context.Background(), env, "HEAD")
	require.NoError(t, err)
	require.Equal(t, "rules", recipe.ReviewRuleset)
	require.Equal(t, "overview", recipe.OverviewGuidance)
	require.Equal(t, "reply", recipe.ReplyCommentGuidance)
	require.Equal(t, "autogen", recipe.AutogenGuidance)
	require.NotNil(t, recipe.ReviewEnabled)
	require.False(t, *recipe.ReviewEnabled)
	require.NotNil(t, recipe.ReviewSuggestions)
	require.True(t, *recipe.ReviewSuggestions)
	require.NotNil(t, recipe.OverviewEnabled)
	require.False(t, *recipe.OverviewEnabled)
	require.NotNil(t, recipe.OverviewIssueAlignmentEnabled)
	require.False(t, *recipe.OverviewIssueAlignmentEnabled)
	require.NotNil(t, recipe.ReplyCommentEnabled)
	require.True(t, *recipe.ReplyCommentEnabled)
	require.NotNil(t, recipe.AutogenEnabled)
	require.True(t, *recipe.AutogenEnabled)
	require.Equal(t, []string{"opened", "synchronize"}, recipe.ReviewEvents)
	require.Equal(t, []string{"opened"}, recipe.OverviewEvents)
	require.Equal(t, []string{"issue_comment", "pull_request_review_comment"}, recipe.ReplyCommentEvents)
	require.Equal(t, []string{"created", "edited"}, recipe.ReplyCommentActions)
	require.Equal(t, []string{"opened"}, recipe.AutogenEvents)
	require.NotNil(t, recipe.AutogenDocs)
	require.True(t, *recipe.AutogenDocs)
	require.NotNil(t, recipe.AutogenTests)
	require.False(t, *recipe.AutogenTests)
	require.Equal(t, "alignment", recipe.OverviewIssueAlignmentGuidance)
}

func TestLoaderIgnoresInvalidRecipePath(t *testing.T) {
	loader, err := NewLoader(&recipeTestSanitizer{status: domain.PromptSafetyStatusOK}, &recipeTestSanitizer{status: domain.PromptSafetyStatusOK}, nil)
	require.NoError(t, err)

	env := &recipeTestEnvironment{files: map[string]string{
		".peer/config.toml": `
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
		".peer/config.toml": `
[review]
ruleset = "rules.md"

[overview]
enabled = true
extra_guidance = "overview.md"
`,
	}}

	recipe, err := loader.Load(context.Background(), env, "HEAD")
	require.NoError(t, err)
	require.ElementsMatch(t, []string{".peer/rules.md", ".peer/overview.md"}, recipe.MissingPaths)
}

func TestLoaderUsesEnvDefaultsWhenConfigMissing(t *testing.T) {
	t.Setenv(envReviewEnabled, "true")
	t.Setenv(envOverviewEvents, "opened, synchronize")
	t.Setenv(envReplyCommentActions, "")

	loader, err := NewLoader(&recipeTestSanitizer{status: domain.PromptSafetyStatusOK}, &recipeTestSanitizer{status: domain.PromptSafetyStatusOK}, nil)
	require.NoError(t, err)

	recipe, err := loader.Load(context.Background(), &recipeTestEnvironment{files: map[string]string{}}, "HEAD")
	require.NoError(t, err)
	require.NotNil(t, recipe.ReviewEnabled)
	require.True(t, *recipe.ReviewEnabled)
	require.Equal(t, []string{"opened", "synchronize"}, recipe.OverviewEvents)
	require.NotNil(t, recipe.ReplyCommentActions)
	require.Len(t, recipe.ReplyCommentActions, 0)
}

func TestLoaderConfigOverridesEnvWithExplicitEmpty(t *testing.T) {
	t.Setenv(envOverviewEvents, "opened")

	loader, err := NewLoader(&recipeTestSanitizer{status: domain.PromptSafetyStatusOK}, &recipeTestSanitizer{status: domain.PromptSafetyStatusOK}, nil)
	require.NoError(t, err)

	env := &recipeTestEnvironment{files: map[string]string{
		".peer/config.toml": `
[overview]
events = []
extra_guidance = ""
`,
	}}

	recipe, err := loader.Load(context.Background(), env, "HEAD")
	require.NoError(t, err)
	require.NotNil(t, recipe.OverviewEvents)
	require.Len(t, recipe.OverviewEvents, 0)
}
