package codingagent

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/bentos-lab/peer/domain"
	"github.com/bentos-lab/peer/usecase"
	"github.com/bentos-lab/peer/usecase/contracts"

	"github.com/stretchr/testify/require"
)

type reviewerTestAgent struct {
	tasks            []string
	opts             []domain.CodingAgentRunOptions
	runCalls         int
	sessionIDToReply string
}

func (a *reviewerTestAgent) Run(_ context.Context, task string, opts domain.CodingAgentRunOptions) (domain.CodingAgentRunResult, error) {
	a.runCalls++
	a.tasks = append(a.tasks, task)
	a.opts = append(a.opts, opts)
	return domain.CodingAgentRunResult{Text: "raw-review-output", SessionID: a.sessionIDToReply}, nil
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

func (e *reviewerTestEnvironment) ResolveBaseHead(_ context.Context, base string, head string) (string, string, error) {
	return base, head, nil
}

func (e *reviewerTestEnvironment) LoadChangedFiles(_ context.Context, _ domain.CodeEnvironmentLoadOptions) ([]domain.ChangedFile, error) {
	e.loadChangedCalls++
	if e.loadChangedErr != nil {
		return nil, e.loadChangedErr
	}
	return e.changedFiles, nil
}

func (e *reviewerTestEnvironment) EnsureDiffContentAvailable(ctx context.Context, opts domain.CodeEnvironmentLoadOptions) error {
	changedFiles, err := e.LoadChangedFiles(ctx, opts)
	if err != nil {
		return err
	}
	for _, file := range changedFiles {
		if strings.TrimSpace(file.DiffSnippet) != "" {
			return nil
		}
	}
	return fmt.Errorf("diff content is empty for base %q and head %q", opts.Base, opts.Head)
}

func (e *reviewerTestEnvironment) ReadFile(_ context.Context, _ string, _ string) (string, bool, error) {
	return "", false, nil
}

func (e *reviewerTestEnvironment) CommitChanges(_ context.Context, _ domain.CodeEnvironmentCommitOptions) (domain.CodeEnvironmentCommitResult, error) {
	return domain.CodeEnvironmentCommitResult{}, nil
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
		agent:        &reviewerTestAgent{sessionIDToReply: "ses_1"},
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
		Suggestions: true,
	})
	require.NoError(t, err)
	require.Equal(t, 1, env.loadChangedCalls)
	require.Equal(t, 2, env.agent.runCalls)
	require.Len(t, env.agent.tasks, 2)

	require.Contains(t, env.agent.tasks[0], "Review the diff between two refs and fix important issues in the changed code when needed.")
	require.Contains(t, env.agent.tasks[0], "Refs:")
	require.Contains(t, env.agent.tasks[0], "Base: main")
	require.Contains(t, env.agent.tasks[0], "Head: feature")
	require.Contains(t, env.agent.tasks[0], "git rev-parse --verify \"main^{commit}\"")
	require.Contains(t, env.agent.tasks[0], "git rev-parse --verify \"feature^{commit}\"")
	require.Contains(t, env.agent.tasks[0], "git merge-base \"main\" \"feature\"")
	require.Contains(t, env.agent.tasks[0], "git diff --name-status \"<merge-base>\" \"feature\"")
	require.Contains(t, env.agent.tasks[0], "git diff --unified=0 --no-color \"<merge-base>\" \"feature\"")
	require.Contains(t, env.agent.tasks[1], "Based on the real issues you found and the changed code, construct a structured report.")
	require.Contains(t, env.agent.tasks[1], "One finding block per issue using consistent labels.")
	require.Contains(t, env.agent.tasks[1], "Also include the changed code (`suggested_change`) for every finding:")

	require.Equal(t, "raw-review-output", formatter.lastArgs.Messages[0])
	require.NotEmpty(t, formatter.lastArgs.SystemPrompt)
	require.Contains(t, formatter.lastArgs.SystemPrompt, "Convert the user-provided reviewer free-form text into strict JSON that matches the provided response schema.")
	require.Contains(t, formatter.lastArgs.SystemPrompt, "You can do:")
	require.Contains(t, formatter.lastArgs.SystemPrompt, "You cannot do:")
	require.NotEqual(t, "You convert reviewer free-form text into strict JSON following the provided schema. Preserve only grounded, explicit findings and keep output concise.", formatter.lastArgs.SystemPrompt)
	require.Len(t, result.Findings, 2)
	require.NotNil(t, result.Findings[0].SuggestedChange)
	require.Equal(t, domain.SuggestedChangeKindReplace, result.Findings[0].SuggestedChange.Kind)
	require.Nil(t, result.Findings[1].SuggestedChange)
	require.Len(t, env.agent.opts, 2)
	require.Equal(t, "", env.agent.opts[0].SessionID)
	require.Equal(t, "ses_1", env.agent.opts[1].SessionID)
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
		Suggestions: false,
	})
	require.NoError(t, err)
	require.Equal(t, 2, env.agent.runCalls)
	require.Len(t, env.agent.tasks, 2)
	require.Contains(t, env.agent.tasks[0], "Refs:")
	require.Contains(t, env.agent.tasks[0], "Base: main")
	require.Contains(t, env.agent.tasks[0], "Head: feature")
	require.NotContains(t, env.agent.tasks[1], "Also include the changed code (`suggested_change`) for every finding:")
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
		Suggestions: false,
	})
	require.ErrorContains(t, err, "base ref is required")
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
		Suggestions: false,
	})
	require.ErrorContains(t, err, "head ref is required")
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
		Suggestions: false,
	})
	require.ErrorContains(t, err, "base ref is required")
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
		Suggestions: false,
	})
	require.ErrorContains(t, err, "head ref must not use workspace tokens")
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
		Suggestions: false,
	})
	require.ErrorContains(t, err, "head ref must not use workspace tokens")
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
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "diff content is empty")
	require.Equal(t, 1, env.loadChangedCalls)
	require.Equal(t, 0, env.setupCalls)
	require.Equal(t, 0, env.agent.runCalls)
}
