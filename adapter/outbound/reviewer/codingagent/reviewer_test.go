package codingagent

import (
	"context"
	"testing"

	"bentos-backend/domain"
	"bentos-backend/usecase"
	"bentos-backend/usecase/contracts"

	"github.com/stretchr/testify/require"
)

type reviewerTestAgent struct {
	lastTask string
	runCalls int
}

func (a *reviewerTestAgent) Run(_ context.Context, task string, _ domain.CodingAgentRunOptions) (domain.CodingAgentRunResult, error) {
	a.runCalls++
	a.lastTask = task
	return domain.CodingAgentRunResult{Text: "raw-review-output"}, nil
}

type reviewerTestEnvironment struct {
	agent            *reviewerTestAgent
	loadChangedCalls int
	setupCalls       int
	changedFiles     []domain.ChangedFile
	loadChangedErr   error
}

func (e *reviewerTestEnvironment) SetupAgent(_ context.Context, _ domain.CodingAgentSetupOptions) (contracts.CodingAgent, error) {
	e.setupCalls++
	return e.agent, nil
}

func (e *reviewerTestEnvironment) LoadChangedFiles(_ context.Context, _ domain.CodeEnvironmentLoadOptions) ([]domain.ChangedFile, error) {
	e.loadChangedCalls++
	if e.loadChangedErr != nil {
		return nil, e.loadChangedErr
	}
	return e.changedFiles, nil
}

func (e *reviewerTestEnvironment) ReadFile(_ context.Context, _ string, _ string) (string, bool, error) {
	return "", false, nil
}

func (e *reviewerTestEnvironment) PushChanges(_ context.Context, _ domain.CodeEnvironmentPushOptions) (domain.CodeEnvironmentPushResult, error) {
	return domain.CodeEnvironmentPushResult{}, nil
}

func (e *reviewerTestEnvironment) Cleanup(_ context.Context) error {
	return nil
}

type reviewerTestFormatter struct {
	output     map[string]any
	lastArgs   contracts.GenerateParams
	lastSchema map[string]any
}

func (f *reviewerTestFormatter) Generate(_ context.Context, _ contracts.GenerateParams) (string, error) {
	return "", nil
}

func (f *reviewerTestFormatter) GenerateJSON(_ context.Context, params contracts.GenerateParams, schema map[string]any) (map[string]any, error) {
	f.lastArgs = params
	f.lastSchema = schema
	return f.output, nil
}

func TestReviewerReviewUsesTaskPromptAndNormalizesSuggestedChanges(t *testing.T) {
	env := &reviewerTestEnvironment{
		agent:        &reviewerTestAgent{},
		changedFiles: []domain.ChangedFile{{Path: "a.go", DiffSnippet: "@@ -1 +1 @@\n-old\n+new"}},
	}
	formatter := &reviewerTestFormatter{output: map[string]any{
		"summary": "summary",
		"findings": []any{
			map[string]any{
				"filePath":   "a.go",
				"startLine":  10,
				"endLine":    12,
				"severity":   "MAJOR",
				"title":      "t1",
				"detail":     "d1",
				"suggestion": "s1",
				"suggestedChange": map[string]any{
					"startLine":   30,
					"endLine":     31,
					"kind":        "REPLACE",
					"replacement": "new()",
					"reason":      "fix",
				},
			},
			map[string]any{
				"filePath":   "b.go",
				"startLine":  20,
				"endLine":    21,
				"severity":   "MINOR",
				"title":      "t2",
				"detail":     "d2",
				"suggestion": "s2",
				"suggestedChange": map[string]any{
					"startLine":   40,
					"endLine":     41,
					"kind":        "DELETE",
					"replacement": "must-be-empty",
					"reason":      "cleanup",
				},
			},
		},
	}}

	reviewer, err := NewReviewer(formatter, Config{Agent: "opencode", Provider: "openai", Model: "m"}, nil)
	require.NoError(t, err)

	result, err := reviewer.Review(context.Background(), usecase.LLMReviewPayload{
		Environment: env,
		Input: domain.ChangeRequestInput{
			Target:      domain.ChangeRequestTarget{Repository: "org/repo"},
			RepoURL:     "https://example.com/repo.git",
			Base:        "main",
			Head:        "feature",
			Title:       "Improve parser",
			Description: "PR desc",
		},
		RulePack:    usecase.RulePack{Instructions: []string{"rule-1", "rule-2"}},
		Suggestions: true,
	})
	require.NoError(t, err)
	require.Equal(t, 1, env.loadChangedCalls)

	require.Contains(t, env.agent.lastTask, "Repository: org/repo")
	require.Contains(t, env.agent.lastTask, "Base: main")
	require.Contains(t, env.agent.lastTask, "Head: feature")
	require.Contains(t, env.agent.lastTask, "Language: English")
	require.Contains(t, env.agent.lastTask, "Include Suggested Changes: true")
	require.Contains(t, env.agent.lastTask, "Base and Head are the canonical comparison anchors; use both whenever available.")
	require.Contains(t, env.agent.lastTask, "git rev-parse --verify \"main^{commit}\"")
	require.Contains(t, env.agent.lastTask, "git rev-parse --verify \"feature^{commit}\"")
	require.Contains(t, env.agent.lastTask, "git merge-base \"main\" \"feature\"")
	require.Contains(t, env.agent.lastTask, "git diff --name-status \"<merge-base>\" \"feature\"")
	require.Contains(t, env.agent.lastTask, "git diff --unified=0 --no-color \"<merge-base>\" \"feature\"")
	require.NotContains(t, env.agent.lastTask, "Base is empty; fallback to head-only inspection.")
	require.NotContains(t, env.agent.lastTask, "Head is empty; fallback to base-only inspection.")
	require.NotContains(t, env.agent.lastTask, "Base and Head are empty; treat as full workspace mode with Base=`HEAD` and Head=`@all`.")
	require.Contains(t, env.agent.lastTask, "Do not group findings by file or category; output a direct finding list only.")
	require.Contains(t, env.agent.lastTask, "changed-code line range (`start-end`)")
	require.Contains(t, env.agent.lastTask, "diff-grounded evidence")
	require.Contains(t, env.agent.lastTask, "line range (`start-end`) for the suggested change target")
	require.Contains(t, env.agent.lastTask, "`kind`: `replace` or `delete`")
	require.Contains(t, env.agent.lastTask, "`replacement`: contains the FULL code (or comment) for replace the old code in `start`-`end` line range, including old lines if those lines don't need to be replaced. Do not include free text in this field. This field is required for `replace`, must be empty for `delete`.")
	require.NotContains(t, env.agent.lastTask, "Explicitly set `suggested_change: none` for every finding.")
	require.NotContains(t, env.agent.lastTask, "```diff")

	require.Equal(t, "raw-review-output", formatter.lastArgs.Messages[0])
	require.NotEmpty(t, formatter.lastArgs.SystemPrompt)
	require.Contains(t, formatter.lastArgs.SystemPrompt, "You are a JSON formatter only.")
	require.Contains(t, formatter.lastArgs.SystemPrompt, "You can do:")
	require.Contains(t, formatter.lastArgs.SystemPrompt, "You cannot do:")
	require.Contains(t, formatter.lastArgs.SystemPrompt, "`summary`")
	require.Contains(t, formatter.lastArgs.SystemPrompt, "`findings[].filePath`")
	require.Contains(t, formatter.lastArgs.SystemPrompt, "`findings[].startLine`")
	require.Contains(t, formatter.lastArgs.SystemPrompt, "`findings[].endLine`")
	require.Contains(t, formatter.lastArgs.SystemPrompt, "`findings[].severity`")
	require.Contains(t, formatter.lastArgs.SystemPrompt, "`findings[].title`")
	require.Contains(t, formatter.lastArgs.SystemPrompt, "`findings[].detail`")
	require.Contains(t, formatter.lastArgs.SystemPrompt, "`findings[].suggestion`")
	require.Contains(t, formatter.lastArgs.SystemPrompt, "`findings[].suggestedChange.kind`")
	require.Contains(t, formatter.lastArgs.SystemPrompt, "`findings[].suggestedChange.startLine`")
	require.Contains(t, formatter.lastArgs.SystemPrompt, "`findings[].suggestedChange.endLine`")
	require.Contains(t, formatter.lastArgs.SystemPrompt, "`findings[].suggestedChange.replacement`")
	require.Contains(t, formatter.lastArgs.SystemPrompt, "`findings[].suggestedChange.reason`")
	require.NotEqual(t, "You convert reviewer free-form text into strict JSON following the provided schema. Preserve only grounded, explicit findings and keep output concise.", formatter.lastArgs.SystemPrompt)
	require.Len(t, result.Findings, 2)
	require.NotNil(t, result.Findings[0].SuggestedChange)
	require.Equal(t, domain.SuggestedChangeKindReplace, result.Findings[0].SuggestedChange.Kind)
	require.Nil(t, result.Findings[1].SuggestedChange)
}

func TestReviewerReviewSuggestionsDisabledDoesNotRequireSuggestedChange(t *testing.T) {
	env := &reviewerTestEnvironment{
		agent:        &reviewerTestAgent{},
		changedFiles: []domain.ChangedFile{{Path: "a.go", DiffSnippet: "@@ -1 +1 @@\n-old\n+new"}},
	}
	formatter := &reviewerTestFormatter{output: map[string]any{
		"summary": "summary",
		"findings": []any{
			map[string]any{
				"filePath":   "a.go",
				"startLine":  1,
				"endLine":    1,
				"severity":   "NIT",
				"title":      "title",
				"detail":     "detail",
				"suggestion": "suggestion",
			},
		},
	}}

	reviewer, err := NewReviewer(formatter, Config{Agent: "opencode", Provider: "openai", Model: "m"}, nil)
	require.NoError(t, err)

	_, err = reviewer.Review(context.Background(), usecase.LLMReviewPayload{
		Environment: env,
		Input: domain.ChangeRequestInput{
			Target:      domain.ChangeRequestTarget{Repository: "org/repo"},
			RepoURL:     "https://example.com/repo.git",
			Base:        "main",
			Head:        "feature",
			Language:    "Vietnamese",
			Title:       "t",
			Description: "d",
		},
		RulePack:    usecase.RulePack{Instructions: []string{"rule"}},
		Suggestions: false,
	})
	require.NoError(t, err)
	require.Contains(t, env.agent.lastTask, "Include Suggested Changes: false")
	require.Contains(t, env.agent.lastTask, "Language: Vietnamese")
	require.Contains(t, env.agent.lastTask, "- NEVER suggest changes for any finding. Your task is JUST analyze and find them.")
	require.NotContains(t, env.agent.lastTask, "`kind`: `replace` or `delete`")
	require.NotContains(t, env.agent.lastTask, "`replacement`: required for `replace`, must be empty for `delete`")
}

func TestReviewerReviewDropsSuggestedChangeWhenRangeIsInvalid(t *testing.T) {
	env := &reviewerTestEnvironment{
		agent:        &reviewerTestAgent{},
		changedFiles: []domain.ChangedFile{{Path: "a.go", DiffSnippet: "@@ -1 +1 @@\n-old\n+new"}},
	}
	formatter := &reviewerTestFormatter{output: map[string]any{
		"summary": "summary",
		"findings": []any{
			map[string]any{
				"filePath":   "a.go",
				"startLine":  10,
				"endLine":    12,
				"severity":   "MAJOR",
				"title":      "t1",
				"detail":     "d1",
				"suggestion": "s1",
				"suggestedChange": map[string]any{
					"startLine":   15,
					"endLine":     14,
					"kind":        "REPLACE",
					"replacement": "new()",
					"reason":      "fix",
				},
			},
		},
	}}

	reviewer, err := NewReviewer(formatter, Config{Agent: "opencode", Provider: "openai", Model: "m"}, nil)
	require.NoError(t, err)

	result, err := reviewer.Review(context.Background(), usecase.LLMReviewPayload{
		Environment: env,
		Input: domain.ChangeRequestInput{
			Target:      domain.ChangeRequestTarget{Repository: "org/repo"},
			RepoURL:     "https://example.com/repo.git",
			Base:        "main",
			Head:        "feature",
			Title:       "Improve parser",
			Description: "PR desc",
		},
		RulePack:    usecase.RulePack{Instructions: []string{"rule-1", "rule-2"}},
		Suggestions: true,
	})
	require.NoError(t, err)
	require.Len(t, result.Findings, 1)
	require.Nil(t, result.Findings[0].SuggestedChange)
}

func TestReviewerReviewTaskPromptBaseEmptyUsesHeadFallback(t *testing.T) {
	env := &reviewerTestEnvironment{
		agent:        &reviewerTestAgent{},
		changedFiles: []domain.ChangedFile{{Path: "a.go", DiffSnippet: "@@ -1 +1 @@\n-old\n+new"}},
	}
	formatter := &reviewerTestFormatter{output: map[string]any{
		"summary":  "summary",
		"findings": []any{},
	}}

	reviewer, err := NewReviewer(formatter, Config{Agent: "opencode", Provider: "openai", Model: "m"}, nil)
	require.NoError(t, err)

	_, err = reviewer.Review(context.Background(), usecase.LLMReviewPayload{
		Environment: env,
		Input: domain.ChangeRequestInput{
			Target:  domain.ChangeRequestTarget{Repository: "org/repo"},
			RepoURL: "https://example.com/repo.git",
			Base:    "",
			Head:    "feature",
		},
		RulePack:    usecase.RulePack{Instructions: []string{"rule"}},
		Suggestions: false,
	})
	require.NoError(t, err)
	require.Contains(t, env.agent.lastTask, "Base is empty; fallback to head-only inspection.")
	require.Contains(t, env.agent.lastTask, "git rev-parse --verify \"feature^{commit}\"")
	require.Contains(t, env.agent.lastTask, "git show --name-status --no-color \"feature\"")
	require.Contains(t, env.agent.lastTask, "git show --unified=0 --no-color \"feature\"")
	require.NotContains(t, env.agent.lastTask, "git diff --name-status \"\" \"feature\"")
	require.NotContains(t, env.agent.lastTask, "git diff --unified=0 --no-color \"\" \"feature\"")
}

func TestReviewerReviewTaskPromptHeadEmptyUsesBaseFallback(t *testing.T) {
	env := &reviewerTestEnvironment{
		agent:        &reviewerTestAgent{},
		changedFiles: []domain.ChangedFile{{Path: "a.go", DiffSnippet: "@@ -1 +1 @@\n-old\n+new"}},
	}
	formatter := &reviewerTestFormatter{output: map[string]any{
		"summary":  "summary",
		"findings": []any{},
	}}

	reviewer, err := NewReviewer(formatter, Config{Agent: "opencode", Provider: "openai", Model: "m"}, nil)
	require.NoError(t, err)

	_, err = reviewer.Review(context.Background(), usecase.LLMReviewPayload{
		Environment: env,
		Input: domain.ChangeRequestInput{
			Target:  domain.ChangeRequestTarget{Repository: "org/repo"},
			RepoURL: "https://example.com/repo.git",
			Base:    "main",
			Head:    "",
		},
		RulePack:    usecase.RulePack{Instructions: []string{"rule"}},
		Suggestions: false,
	})
	require.NoError(t, err)
	require.Contains(t, env.agent.lastTask, "Head is empty; fallback to base-only inspection.")
	require.Contains(t, env.agent.lastTask, "git rev-parse --verify \"main^{commit}\"")
	require.Contains(t, env.agent.lastTask, "git show --name-status --no-color \"main\"")
	require.Contains(t, env.agent.lastTask, "git show --unified=0 --no-color \"main\"")
	require.NotContains(t, env.agent.lastTask, "git diff --name-status \"main\" \"\"")
	require.NotContains(t, env.agent.lastTask, "git diff --unified=0 --no-color \"main\" \"\"")
}

func TestReviewerReviewTaskPromptBaseAndHeadEmptyUsesWorkspaceFallback(t *testing.T) {
	env := &reviewerTestEnvironment{
		agent:        &reviewerTestAgent{},
		changedFiles: []domain.ChangedFile{{Path: "a.go", DiffSnippet: "@@ -1 +1 @@\n-old\n+new"}},
	}
	formatter := &reviewerTestFormatter{output: map[string]any{
		"summary":  "summary",
		"findings": []any{},
	}}

	reviewer, err := NewReviewer(formatter, Config{Agent: "opencode", Provider: "openai", Model: "m"}, nil)
	require.NoError(t, err)

	_, err = reviewer.Review(context.Background(), usecase.LLMReviewPayload{
		Environment: env,
		Input: domain.ChangeRequestInput{
			Target:  domain.ChangeRequestTarget{Repository: "org/repo"},
			RepoURL: "https://example.com/repo.git",
			Base:    "",
			Head:    "",
		},
		RulePack:    usecase.RulePack{Instructions: []string{"rule"}},
		Suggestions: false,
	})
	require.NoError(t, err)
	require.Contains(t, env.agent.lastTask, "Base and Head are empty; treat as full workspace mode with Base=`HEAD` and Head=`@all`.")
	require.Contains(t, env.agent.lastTask, "git diff --cached --name-status")
	require.Contains(t, env.agent.lastTask, "git diff --cached --unified=0 --no-color")
	require.NotContains(t, env.agent.lastTask, "git rev-parse --verify \"^{commit}\"")
}

func TestReviewerReviewTaskPromptStagedTokenUsesStagedWorkspaceMode(t *testing.T) {
	env := &reviewerTestEnvironment{
		agent:        &reviewerTestAgent{},
		changedFiles: []domain.ChangedFile{{Path: "a.go", DiffSnippet: "@@ -1 +1 @@\n-old\n+new"}},
	}
	formatter := &reviewerTestFormatter{output: map[string]any{
		"summary":  "summary",
		"findings": []any{},
	}}

	reviewer, err := NewReviewer(formatter, Config{Agent: "opencode", Provider: "openai", Model: "m"}, nil)
	require.NoError(t, err)

	_, err = reviewer.Review(context.Background(), usecase.LLMReviewPayload{
		Environment: env,
		Input: domain.ChangeRequestInput{
			Target:  domain.ChangeRequestTarget{Repository: "org/repo"},
			RepoURL: "https://example.com/repo.git",
			Base:    "HEAD",
			Head:    "@staged",
		},
		RulePack:    usecase.RulePack{Instructions: []string{"rule"}},
		Suggestions: false,
	})
	require.NoError(t, err)
	require.Contains(t, env.agent.lastTask, "Head uses staged workspace mode.")
	require.Contains(t, env.agent.lastTask, "git diff --cached --name-status")
	require.Contains(t, env.agent.lastTask, "git diff --cached --unified=0 --no-color")
	require.NotContains(t, env.agent.lastTask, "git rev-parse --verify")
	require.NotContains(t, env.agent.lastTask, "git diff --name-status \"HEAD\" \"@staged\"")
}

func TestReviewerReviewTaskPromptAllTokenUsesFullWorkspaceMode(t *testing.T) {
	env := &reviewerTestEnvironment{
		agent:        &reviewerTestAgent{},
		changedFiles: []domain.ChangedFile{{Path: "a.go", DiffSnippet: "@@ -1 +1 @@\n-old\n+new"}},
	}
	formatter := &reviewerTestFormatter{output: map[string]any{
		"summary":  "summary",
		"findings": []any{},
	}}

	reviewer, err := NewReviewer(formatter, Config{Agent: "opencode", Provider: "openai", Model: "m"}, nil)
	require.NoError(t, err)

	_, err = reviewer.Review(context.Background(), usecase.LLMReviewPayload{
		Environment: env,
		Input: domain.ChangeRequestInput{
			Target:  domain.ChangeRequestTarget{Repository: "org/repo"},
			RepoURL: "https://example.com/repo.git",
			Base:    "HEAD",
			Head:    "@all",
		},
		RulePack:    usecase.RulePack{Instructions: []string{"rule"}},
		Suggestions: false,
	})
	require.NoError(t, err)
	require.Contains(t, env.agent.lastTask, "Head uses full workspace mode (staged + unstaged + untracked).")
	require.Contains(t, env.agent.lastTask, "git diff --cached --name-status")
	require.Contains(t, env.agent.lastTask, "git diff --name-status")
	require.Contains(t, env.agent.lastTask, "git ls-files --others --exclude-standard")
	require.Contains(t, env.agent.lastTask, "git diff --cached --unified=0 --no-color")
	require.Contains(t, env.agent.lastTask, "git diff --unified=0 --no-color")
	require.NotContains(t, env.agent.lastTask, "git rev-parse --verify")
}

func TestReviewerReviewReturnsErrorWhenEnvironmentMissing(t *testing.T) {
	formatter := &reviewerTestFormatter{output: map[string]any{
		"summary":  "summary",
		"findings": []any{},
	}}
	reviewer, err := NewReviewer(formatter, Config{Agent: "opencode", Provider: "openai", Model: "m"}, nil)
	require.NoError(t, err)

	_, err = reviewer.Review(context.Background(), usecase.LLMReviewPayload{
		Input: domain.ChangeRequestInput{
			Target: domain.ChangeRequestTarget{Repository: "org/repo"},
		},
		RulePack: usecase.RulePack{Instructions: []string{"rule"}},
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "code environment must not be nil")
}

func TestReviewerReviewReturnsErrorWhenDiffContentIsEmpty(t *testing.T) {
	env := &reviewerTestEnvironment{
		agent:        &reviewerTestAgent{},
		changedFiles: []domain.ChangedFile{{Path: "a.go", DiffSnippet: "   "}},
	}
	formatter := &reviewerTestFormatter{output: map[string]any{
		"summary":  "summary",
		"findings": []any{},
	}}

	reviewer, err := NewReviewer(formatter, Config{Agent: "opencode", Provider: "openai", Model: "m"}, nil)
	require.NoError(t, err)

	_, err = reviewer.Review(context.Background(), usecase.LLMReviewPayload{
		Environment: env,
		Input: domain.ChangeRequestInput{
			Target:  domain.ChangeRequestTarget{Repository: "org/repo"},
			RepoURL: "https://example.com/repo.git",
			Base:    "main",
			Head:    "feature",
		},
		RulePack: usecase.RulePack{Instructions: []string{"rule"}},
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "diff content is empty")
	require.Equal(t, 1, env.loadChangedCalls)
	require.Equal(t, 0, env.setupCalls)
	require.Equal(t, 0, env.agent.runCalls)
}
