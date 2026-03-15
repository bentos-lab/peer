package codeagent

import (
	"context"
	"testing"

	"bentos-backend/domain"
	"bentos-backend/usecase"
	"bentos-backend/usecase/contracts"

	"github.com/stretchr/testify/require"
)

type issueAlignmentTestAgent struct {
	lastTask string
	runCalls int
}

func (a *issueAlignmentTestAgent) Run(_ context.Context, task string, _ domain.CodingAgentRunOptions) (domain.CodingAgentRunResult, error) {
	a.runCalls++
	a.lastTask = task
	return domain.CodingAgentRunResult{Text: "raw-issue-alignment-output"}, nil
}

type issueAlignmentTestEnvironment struct {
	agent        *issueAlignmentTestAgent
	changedFiles []domain.ChangedFile
	setupCalls   int
	lastSetup    domain.CodingAgentSetupOptions
}

func (e *issueAlignmentTestEnvironment) SetupAgent(_ context.Context, opts domain.CodingAgentSetupOptions) (contracts.CodingAgent, error) {
	e.setupCalls++
	e.lastSetup = opts
	return e.agent, nil
}

func (e *issueAlignmentTestEnvironment) LoadChangedFiles(_ context.Context, _ domain.CodeEnvironmentLoadOptions) ([]domain.ChangedFile, error) {
	return e.changedFiles, nil
}

func (e *issueAlignmentTestEnvironment) ReadFile(_ context.Context, _ string, _ string) (string, bool, error) {
	return "", false, nil
}

func (e *issueAlignmentTestEnvironment) PushChanges(_ context.Context, _ domain.CodeEnvironmentPushOptions) (domain.CodeEnvironmentPushResult, error) {
	return domain.CodeEnvironmentPushResult{}, nil
}

func (e *issueAlignmentTestEnvironment) Cleanup(_ context.Context) error {
	return nil
}

type issueAlignmentTestFormatter struct {
	generateCalls     int
	generateJSONCalls int
	lastGenerateArgs  contracts.GenerateParams
	lastJSONArgs      []contracts.GenerateParams
	keyIdeasOutput    map[string]any
	alignmentOutput   map[string]any
}

func (g *issueAlignmentTestFormatter) Generate(_ context.Context, params contracts.GenerateParams) (string, error) {
	g.generateCalls++
	g.lastGenerateArgs = params
	return "", nil
}

func (g *issueAlignmentTestFormatter) GenerateJSON(_ context.Context, params contracts.GenerateParams, _ map[string]any) (map[string]any, error) {
	g.generateJSONCalls++
	g.lastJSONArgs = append(g.lastJSONArgs, params)
	if g.generateJSONCalls == 1 {
		return g.keyIdeasOutput, nil
	}
	return g.alignmentOutput, nil
}

func TestGenerateIssueAlignmentUsesKeyIdeas(t *testing.T) {
	formatter := &issueAlignmentTestFormatter{
		keyIdeasOutput: map[string]any{
			"keyIdeas": []any{"Requirement A", "Requirement A", "Requirement B"},
		},
		alignmentOutput: map[string]any{
			"issue":    map[string]any{"repository": "org/repo", "number": 9, "title": "Issue"},
			"keyIdeas": []any{"Requirement A"},
			"requirements": []any{
				map[string]any{"requirement": "Requirement A", "coverage": "Explained coverage"},
			},
		},
	}
	agent := &issueAlignmentTestAgent{}
	env := &issueAlignmentTestEnvironment{
		agent:        agent,
		changedFiles: []domain.ChangedFile{{Path: "a.go", DiffSnippet: "@@ -1 +1 @@\n-old\n+new"}},
	}

	alignmentGenerator, err := NewIssueAlignmentGenerator(formatter, Config{Agent: "opencode", Provider: "openai", Model: "m"}, nil)
	require.NoError(t, err)

	result, err := alignmentGenerator.GenerateIssueAlignment(context.Background(), usecase.LLMIssueAlignmentPayload{
		Input: domain.ChangeRequestInput{
			Target:      domain.ChangeRequestTarget{Repository: "org/repo"},
			RepoURL:     "https://example.com/repo.git",
			Base:        "main",
			Head:        "feature",
			Title:       "PR title",
			Description: "PR description",
		},
		IssueAlignment: usecase.OverviewIssueAlignmentInput{
			Candidates: []domain.IssueContext{
				{Issue: domain.Issue{Repository: "org/repo", Number: 9, Title: "Issue"}},
			},
		},
		Environment: env,
	})
	require.NoError(t, err)
	require.Equal(t, []string{"Requirement A", "Requirement B"}, result.KeyIdeas)
	require.Len(t, result.Requirements, 1)
	require.Equal(t, "Requirement A", result.Requirements[0].Requirement)
	require.Equal(t, "Explained coverage", result.Requirements[0].Coverage)
	require.Equal(t, 0, formatter.generateCalls)
	require.Equal(t, 2, formatter.generateJSONCalls)
	require.Equal(t, 1, env.setupCalls)
	require.Equal(t, "opencode", env.lastSetup.Agent)
	require.Equal(t, "feature", env.lastSetup.Ref)
	require.Equal(t, 1, agent.runCalls)
	require.Contains(t, agent.lastTask, "Repository: org/repo")
	require.Contains(t, agent.lastTask, "Key ideas:")
	require.Contains(t, agent.lastTask, "Requirement A")
	require.Contains(t, agent.lastTask, "Requirement B")
	require.Contains(t, agent.lastTask, "DO NOT propose code changes.")
	require.Contains(t, formatter.lastJSONArgs[0].Messages[0], "org/repo#9")
	require.Contains(t, formatter.lastJSONArgs[0].Messages[0], "Issue")
	require.Equal(t, "raw-issue-alignment-output", formatter.lastJSONArgs[1].Messages[0])
}

func TestGenerateIssueAlignmentFallsBackToFirstIssue(t *testing.T) {
	formatter := &issueAlignmentTestFormatter{
		keyIdeasOutput: map[string]any{
			"keyIdeas": []any{"Requirement A"},
		},
		alignmentOutput: map[string]any{
			"issue":    map[string]any{"repository": "", "number": 0, "title": ""},
			"keyIdeas": []any{"Requirement A"},
			"requirements": []any{
				map[string]any{"requirement": "Requirement A", "coverage": "Explained coverage"},
			},
		},
	}
	alignmentGenerator, err := NewIssueAlignmentGenerator(formatter, Config{Agent: "opencode", Provider: "openai", Model: "m"}, nil)
	require.NoError(t, err)

	result, err := alignmentGenerator.GenerateIssueAlignment(context.Background(), usecase.LLMIssueAlignmentPayload{
		Input: domain.ChangeRequestInput{
			Target:  domain.ChangeRequestTarget{Repository: "org/repo"},
			RepoURL: "https://example.com/repo.git",
			Base:    "main",
			Head:    "feature",
		},
		IssueAlignment: usecase.OverviewIssueAlignmentInput{
			Candidates: []domain.IssueContext{
				{Issue: domain.Issue{Repository: "org/repo", Number: 13, Title: "Fallback"}},
			},
		},
		Environment: &issueAlignmentTestEnvironment{agent: &issueAlignmentTestAgent{}},
	})
	require.NoError(t, err)
	require.Equal(t, 13, result.Issue.Number)
	require.Equal(t, "org/repo", result.Issue.Repository)
	require.Equal(t, "Fallback", result.Issue.Title)
}
