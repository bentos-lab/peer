package llm

import (
	"context"
	"testing"

	"bentos-backend/domain"
	"bentos-backend/usecase"
	"bentos-backend/usecase/contracts"

	"github.com/stretchr/testify/require"
)

type issueAlignmentTestEnvironment struct {
	changedFiles []domain.ChangedFile
}

func (e *issueAlignmentTestEnvironment) SetupAgent(_ context.Context, _ domain.CodingAgentSetupOptions) (contracts.CodingAgent, error) {
	return nil, nil
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

type issueAlignmentTestGenerator struct {
	generateCalls     int
	generateJSONCalls int
	lastGenerateArgs  contracts.GenerateParams
	lastJSONArgs      []contracts.GenerateParams
	keyIdeasOutput    map[string]any
	alignmentOutput   map[string]any
	generateOutput    string
}

func (g *issueAlignmentTestGenerator) Generate(_ context.Context, params contracts.GenerateParams) (string, error) {
	g.generateCalls++
	g.lastGenerateArgs = params
	return g.generateOutput, nil
}

func (g *issueAlignmentTestGenerator) GenerateJSON(_ context.Context, params contracts.GenerateParams, _ map[string]any) (map[string]any, error) {
	g.generateJSONCalls++
	g.lastJSONArgs = append(g.lastJSONArgs, params)
	if g.generateJSONCalls == 1 {
		return g.keyIdeasOutput, nil
	}
	return g.alignmentOutput, nil
}

func TestGenerateIssueAlignmentUsesKeyIdeas(t *testing.T) {
	generator := &issueAlignmentTestGenerator{
		keyIdeasOutput: map[string]any{
			"keyIdeas": []any{"Requirement A", "Requirement A", "Requirement B"},
		},
		alignmentOutput: map[string]any{
			"issue":    map[string]any{"repository": "org/repo", "number": 9, "title": "Issue"},
			"keyIdeas": []any{"Requirement A"},
			"requirements": []any{
				map[string]any{"requirement": "Requirement A", "coverage": "Yes"},
			},
		},
		generateOutput: "issue\n- repository: org/repo",
	}
	alignmentGenerator, err := NewIssueAlignmentGenerator(generator, nil)
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
		Environment: &issueAlignmentTestEnvironment{
			changedFiles: []domain.ChangedFile{{Path: "a.go", DiffSnippet: "@@ -1 +1 @@\n-old\n+new"}},
		},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"Requirement A", "Requirement B"}, result.KeyIdeas)
	require.Len(t, result.Requirements, 1)
	require.Equal(t, "Requirement A", result.Requirements[0].Requirement)
	require.Equal(t, "Yes", result.Requirements[0].Coverage)
	require.Equal(t, 1, generator.generateCalls)
	require.Equal(t, 2, generator.generateJSONCalls)
	require.Contains(t, generator.lastGenerateArgs.Messages[0], "Requirement A")
	require.Contains(t, generator.lastGenerateArgs.Messages[0], "Requirement B")
}

func TestGenerateIssueAlignmentFallsBackToFirstIssue(t *testing.T) {
	generator := &issueAlignmentTestGenerator{
		keyIdeasOutput: map[string]any{
			"keyIdeas": []any{"Requirement A"},
		},
		alignmentOutput: map[string]any{
			"issue":    map[string]any{"repository": "", "number": 0, "title": ""},
			"keyIdeas": []any{"Requirement A"},
			"requirements": []any{
				map[string]any{"requirement": "Requirement A", "coverage": "Unknown"},
			},
		},
		generateOutput: "issue\n- repository: org/repo",
	}
	alignmentGenerator, err := NewIssueAlignmentGenerator(generator, nil)
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
		Environment: &issueAlignmentTestEnvironment{},
	})
	require.NoError(t, err)
	require.Equal(t, 13, result.Issue.Number)
	require.Equal(t, "org/repo", result.Issue.Repository)
	require.Equal(t, "Fallback", result.Issue.Title)
}
