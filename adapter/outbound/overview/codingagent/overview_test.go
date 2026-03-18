package codingagent

import (
	"context"
	"testing"

	"github.com/bentos-lab/peer/domain"
	"github.com/bentos-lab/peer/usecase"
	"github.com/bentos-lab/peer/usecase/contracts"

	"github.com/stretchr/testify/require"
)

type overviewTestAgent struct {
	lastTask string
	runCalls int
}

func (a *overviewTestAgent) Run(_ context.Context, task string, _ domain.CodingAgentRunOptions) (domain.CodingAgentRunResult, error) {
	a.runCalls++
	a.lastTask = task
	return domain.CodingAgentRunResult{Text: "raw-overview-output"}, nil
}

type overviewTestEnvironment struct {
	agent            *overviewTestAgent
	loadChangedCalls int
	setupCalls       int
	changedFiles     []domain.ChangedFile
	loadChangedErr   error
}

func (e *overviewTestEnvironment) SetupAgent(_ context.Context, _ domain.CodingAgentSetupOptions) (contracts.CodingAgent, error) {
	e.setupCalls++
	return e.agent, nil
}

func (e *overviewTestEnvironment) LoadChangedFiles(_ context.Context, _ domain.CodeEnvironmentLoadOptions) ([]domain.ChangedFile, error) {
	e.loadChangedCalls++
	if e.loadChangedErr != nil {
		return nil, e.loadChangedErr
	}
	return e.changedFiles, nil
}

func (e *overviewTestEnvironment) ReadFile(_ context.Context, _ string, _ string) (string, bool, error) {
	return "", false, nil
}

func (e *overviewTestEnvironment) PushChanges(_ context.Context, _ domain.CodeEnvironmentPushOptions) (domain.CodeEnvironmentPushResult, error) {
	return domain.CodeEnvironmentPushResult{}, nil
}

func (e *overviewTestEnvironment) Cleanup(_ context.Context) error {
	return nil
}

type overviewTestFormatter struct {
	output     map[string]any
	lastArgs   contracts.GenerateParams
	lastSchema map[string]any
}

func (f *overviewTestFormatter) Generate(_ context.Context, _ contracts.GenerateParams) (string, error) {
	return "", nil
}

func (f *overviewTestFormatter) GenerateJSON(_ context.Context, params contracts.GenerateParams, schema map[string]any) (map[string]any, error) {
	f.lastArgs = params
	f.lastSchema = schema
	return f.output, nil
}

func TestGenerateOverviewUsesTaskPromptWithChangedFiles(t *testing.T) {
	env := &overviewTestEnvironment{
		agent:        &overviewTestAgent{},
		changedFiles: []domain.ChangedFile{{Path: "a.go", DiffSnippet: "@@ -1 +1 @@\n-old\n+new"}},
	}
	formatter := &overviewTestFormatter{output: map[string]any{
		"categories": []any{
			map[string]any{"category": "Logic Updates", "summary": "logic"},
		},
		"walkthroughs": []any{
			map[string]any{"groupName": "core", "files": []any{"a.go"}, "summary": "sum"},
		},
	}}

	generator, err := NewOverviewGenerator(formatter, Config{Agent: "opencode", Provider: "openai", Model: "m"}, nil)
	require.NoError(t, err)

	_, err = generator.GenerateOverview(context.Background(), usecase.LLMOverviewPayload{Environment: env, Input: domain.ChangeRequestInput{
		Target:      domain.ChangeRequestTarget{Repository: "org/repo"},
		RepoURL:     "https://example.com/repo.git",
		Base:        "main",
		Head:        "feature",
		Title:       "Overview title",
		Description: "Overview desc",
	}})
	require.NoError(t, err)
	require.Equal(t, 1, env.loadChangedCalls)
	require.Contains(t, env.agent.lastTask, "Repository: org/repo")
	require.Contains(t, env.agent.lastTask, "Base: main")
	require.Contains(t, env.agent.lastTask, "Head: feature")
	require.Contains(t, env.agent.lastTask, "Base and Head are the canonical comparison anchors; use both whenever available.")
	require.Contains(t, env.agent.lastTask, "git rev-parse --verify \"main^{commit}\"")
	require.Contains(t, env.agent.lastTask, "git rev-parse --verify \"feature^{commit}\"")
	require.Contains(t, env.agent.lastTask, "git merge-base \"main\" \"feature\"")
	require.Contains(t, env.agent.lastTask, "git diff --name-status \"<merge-base>\" \"feature\"")
	require.Contains(t, env.agent.lastTask, "git diff --unified=0 --no-color \"<merge-base>\" \"feature\"")
	require.NotContains(t, env.agent.lastTask, "Base is empty; fallback to head-only inspection.")
	require.NotContains(t, env.agent.lastTask, "Head is empty; fallback to base-only inspection.")
	require.NotContains(t, env.agent.lastTask, "Base and Head are empty; treat as full workspace mode with Base=`HEAD` and Head=`@all`.")
	require.Contains(t, env.agent.lastTask, "Use deterministic section order and stable labels exactly as below:")
	require.Contains(t, env.agent.lastTask, "`summary`")
	require.Contains(t, env.agent.lastTask, "`categories`")
	require.Contains(t, env.agent.lastTask, "`walkthroughs`")
	require.Contains(t, env.agent.lastTask, "diff evidence cue")
	require.NotContains(t, env.agent.lastTask, "```diff")
	require.Equal(t, "raw-overview-output", formatter.lastArgs.Messages[0])
	require.NotEmpty(t, formatter.lastArgs.SystemPrompt)
	require.Contains(t, formatter.lastArgs.SystemPrompt, "You are a JSON formatter only.")
	require.Contains(t, formatter.lastArgs.SystemPrompt, "You can do:")
	require.Contains(t, formatter.lastArgs.SystemPrompt, "You cannot do:")
	require.Contains(t, formatter.lastArgs.SystemPrompt, "`categories[].category`")
	require.Contains(t, formatter.lastArgs.SystemPrompt, "`categories[].summary`")
	require.Contains(t, formatter.lastArgs.SystemPrompt, "`walkthroughs[].groupName`")
	require.Contains(t, formatter.lastArgs.SystemPrompt, "`walkthroughs[].files`")
	require.Contains(t, formatter.lastArgs.SystemPrompt, "`walkthroughs[].summary`")
	require.NotEqual(t, "You convert overview free-form text into strict JSON following the provided schema.", formatter.lastArgs.SystemPrompt)
}

func TestGenerateOverviewRejectsInvalidCategory(t *testing.T) {
	env := &overviewTestEnvironment{
		agent:        &overviewTestAgent{},
		changedFiles: []domain.ChangedFile{{Path: "a.go", DiffSnippet: "@@ -1 +1 @@\n-old\n+new"}},
	}
	formatter := &overviewTestFormatter{output: map[string]any{
		"categories": []any{
			map[string]any{"category": "Invalid", "summary": "logic"},
		},
		"walkthroughs": []any{},
	}}

	generator, err := NewOverviewGenerator(formatter, Config{Agent: "opencode", Provider: "openai", Model: "m"}, nil)
	require.NoError(t, err)

	_, err = generator.GenerateOverview(context.Background(), usecase.LLMOverviewPayload{Environment: env, Input: domain.ChangeRequestInput{
		Target:  domain.ChangeRequestTarget{Repository: "org/repo"},
		RepoURL: "https://example.com/repo.git",
		Base:    "main",
		Head:    "feature",
	}})
	require.Error(t, err)
	require.ErrorContains(t, err, "invalid overview category")
}

func TestGenerateOverviewTaskPromptBaseEmptyUsesHeadFallback(t *testing.T) {
	env := &overviewTestEnvironment{
		agent:        &overviewTestAgent{},
		changedFiles: []domain.ChangedFile{{Path: "a.go", DiffSnippet: "@@ -1 +1 @@\n-old\n+new"}},
	}
	formatter := &overviewTestFormatter{output: map[string]any{
		"categories": []any{
			map[string]any{"category": "Logic Updates", "summary": "logic"},
		},
		"walkthroughs": []any{},
	}}

	generator, err := NewOverviewGenerator(formatter, Config{Agent: "opencode", Provider: "openai", Model: "m"}, nil)
	require.NoError(t, err)

	_, err = generator.GenerateOverview(context.Background(), usecase.LLMOverviewPayload{Environment: env, Input: domain.ChangeRequestInput{
		Target:  domain.ChangeRequestTarget{Repository: "org/repo"},
		RepoURL: "https://example.com/repo.git",
		Base:    "",
		Head:    "feature",
	}})
	require.NoError(t, err)
	require.Contains(t, env.agent.lastTask, "Base is empty; fallback to head-only inspection.")
	require.Contains(t, env.agent.lastTask, "git rev-parse --verify \"feature^{commit}\"")
	require.Contains(t, env.agent.lastTask, "git show --name-status --no-color \"feature\"")
	require.Contains(t, env.agent.lastTask, "git show --unified=0 --no-color \"feature\"")
	require.NotContains(t, env.agent.lastTask, "git diff --name-status \"\" \"feature\"")
}

func TestGenerateOverviewTaskPromptHeadEmptyUsesBaseFallback(t *testing.T) {
	env := &overviewTestEnvironment{
		agent:        &overviewTestAgent{},
		changedFiles: []domain.ChangedFile{{Path: "a.go", DiffSnippet: "@@ -1 +1 @@\n-old\n+new"}},
	}
	formatter := &overviewTestFormatter{output: map[string]any{
		"categories": []any{
			map[string]any{"category": "Logic Updates", "summary": "logic"},
		},
		"walkthroughs": []any{},
	}}

	generator, err := NewOverviewGenerator(formatter, Config{Agent: "opencode", Provider: "openai", Model: "m"}, nil)
	require.NoError(t, err)

	_, err = generator.GenerateOverview(context.Background(), usecase.LLMOverviewPayload{Environment: env, Input: domain.ChangeRequestInput{
		Target:  domain.ChangeRequestTarget{Repository: "org/repo"},
		RepoURL: "https://example.com/repo.git",
		Base:    "main",
		Head:    "",
	}})
	require.NoError(t, err)
	require.Contains(t, env.agent.lastTask, "Head is empty; fallback to base-only inspection.")
	require.Contains(t, env.agent.lastTask, "git rev-parse --verify \"main^{commit}\"")
	require.Contains(t, env.agent.lastTask, "git show --name-status --no-color \"main\"")
	require.Contains(t, env.agent.lastTask, "git show --unified=0 --no-color \"main\"")
	require.NotContains(t, env.agent.lastTask, "git diff --name-status \"main\" \"\"")
}

func TestGenerateOverviewTaskPromptBaseAndHeadEmptyUsesWorkspaceFallback(t *testing.T) {
	env := &overviewTestEnvironment{
		agent:        &overviewTestAgent{},
		changedFiles: []domain.ChangedFile{{Path: "a.go", DiffSnippet: "@@ -1 +1 @@\n-old\n+new"}},
	}
	formatter := &overviewTestFormatter{output: map[string]any{
		"categories": []any{
			map[string]any{"category": "Logic Updates", "summary": "logic"},
		},
		"walkthroughs": []any{},
	}}

	generator, err := NewOverviewGenerator(formatter, Config{Agent: "opencode", Provider: "openai", Model: "m"}, nil)
	require.NoError(t, err)

	_, err = generator.GenerateOverview(context.Background(), usecase.LLMOverviewPayload{Environment: env, Input: domain.ChangeRequestInput{
		Target:  domain.ChangeRequestTarget{Repository: "org/repo"},
		RepoURL: "https://example.com/repo.git",
		Base:    "",
		Head:    "",
	}})
	require.NoError(t, err)
	require.Contains(t, env.agent.lastTask, "Base and Head are empty; treat as full workspace mode with Base=`HEAD` and Head=`@all`.")
	require.Contains(t, env.agent.lastTask, "git diff --cached --name-status")
	require.Contains(t, env.agent.lastTask, "git diff --cached --unified=0 --no-color")
}

func TestGenerateOverviewTaskPromptStagedTokenUsesStagedWorkspaceMode(t *testing.T) {
	env := &overviewTestEnvironment{
		agent:        &overviewTestAgent{},
		changedFiles: []domain.ChangedFile{{Path: "a.go", DiffSnippet: "@@ -1 +1 @@\n-old\n+new"}},
	}
	formatter := &overviewTestFormatter{output: map[string]any{
		"categories": []any{
			map[string]any{"category": "Logic Updates", "summary": "logic"},
		},
		"walkthroughs": []any{},
	}}

	generator, err := NewOverviewGenerator(formatter, Config{Agent: "opencode", Provider: "openai", Model: "m"}, nil)
	require.NoError(t, err)

	_, err = generator.GenerateOverview(context.Background(), usecase.LLMOverviewPayload{Environment: env, Input: domain.ChangeRequestInput{
		Target:  domain.ChangeRequestTarget{Repository: "org/repo"},
		RepoURL: "https://example.com/repo.git",
		Base:    "HEAD",
		Head:    "@staged",
	}})
	require.NoError(t, err)
	require.Contains(t, env.agent.lastTask, "Head uses staged workspace mode.")
	require.Contains(t, env.agent.lastTask, "git diff --cached --name-status")
	require.Contains(t, env.agent.lastTask, "git diff --cached --unified=0 --no-color")
	require.NotContains(t, env.agent.lastTask, "git rev-parse --verify")
}

func TestGenerateOverviewTaskPromptAllTokenUsesFullWorkspaceMode(t *testing.T) {
	env := &overviewTestEnvironment{
		agent:        &overviewTestAgent{},
		changedFiles: []domain.ChangedFile{{Path: "a.go", DiffSnippet: "@@ -1 +1 @@\n-old\n+new"}},
	}
	formatter := &overviewTestFormatter{output: map[string]any{
		"categories": []any{
			map[string]any{"category": "Logic Updates", "summary": "logic"},
		},
		"walkthroughs": []any{},
	}}

	generator, err := NewOverviewGenerator(formatter, Config{Agent: "opencode", Provider: "openai", Model: "m"}, nil)
	require.NoError(t, err)

	_, err = generator.GenerateOverview(context.Background(), usecase.LLMOverviewPayload{Environment: env, Input: domain.ChangeRequestInput{
		Target:  domain.ChangeRequestTarget{Repository: "org/repo"},
		RepoURL: "https://example.com/repo.git",
		Base:    "HEAD",
		Head:    "@all",
	}})
	require.NoError(t, err)
	require.Contains(t, env.agent.lastTask, "Head uses full workspace mode (staged + unstaged + untracked).")
	require.Contains(t, env.agent.lastTask, "git diff --cached --name-status")
	require.Contains(t, env.agent.lastTask, "git diff --name-status")
	require.Contains(t, env.agent.lastTask, "git ls-files --others --exclude-standard")
	require.Contains(t, env.agent.lastTask, "git diff --cached --unified=0 --no-color")
	require.Contains(t, env.agent.lastTask, "git diff --unified=0 --no-color")
	require.NotContains(t, env.agent.lastTask, "git rev-parse --verify")
}

func TestGenerateOverviewReturnsErrorWhenEnvironmentMissing(t *testing.T) {
	formatter := &overviewTestFormatter{output: map[string]any{
		"categories":   []any{},
		"walkthroughs": []any{},
	}}
	generator, err := NewOverviewGenerator(formatter, Config{Agent: "opencode", Provider: "openai", Model: "m"}, nil)
	require.NoError(t, err)

	_, err = generator.GenerateOverview(context.Background(), usecase.LLMOverviewPayload{
		Input: domain.ChangeRequestInput{
			Target: domain.ChangeRequestTarget{Repository: "org/repo"},
		},
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "code environment must not be nil")
}

func TestGenerateOverviewReturnsErrorWhenDiffContentIsEmpty(t *testing.T) {
	env := &overviewTestEnvironment{
		agent:        &overviewTestAgent{},
		changedFiles: []domain.ChangedFile{{Path: "a.go", DiffSnippet: "   "}},
	}
	formatter := &overviewTestFormatter{output: map[string]any{
		"categories":   []any{},
		"walkthroughs": []any{},
	}}
	generator, err := NewOverviewGenerator(formatter, Config{Agent: "opencode", Provider: "openai", Model: "m"}, nil)
	require.NoError(t, err)

	_, err = generator.GenerateOverview(context.Background(), usecase.LLMOverviewPayload{Environment: env, Input: domain.ChangeRequestInput{
		Target:  domain.ChangeRequestTarget{Repository: "org/repo"},
		RepoURL: "https://example.com/repo.git",
		Base:    "main",
		Head:    "feature",
	}})
	require.Error(t, err)
	require.ErrorContains(t, err, "diff content is empty")
	require.Equal(t, 1, env.loadChangedCalls)
	require.Equal(t, 0, env.setupCalls)
	require.Equal(t, 0, env.agent.runCalls)
}
