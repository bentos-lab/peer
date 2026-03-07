package llm

import (
	"context"
	"errors"
	"strings"
	"testing"

	"bentos-backend/domain"
	"bentos-backend/usecase"
	"bentos-backend/usecase/contracts"
	"github.com/stretchr/testify/require"
)

type mockGenerator struct {
	result     map[string]any
	err        error
	lastParams contracts.GenerateParams
}

func (m *mockGenerator) Generate(_ context.Context, _ contracts.GenerateParams) (string, error) {
	return "", errors.New("not used")
}

func (m *mockGenerator) GenerateJSON(_ context.Context, params contracts.GenerateParams) (map[string]any, error) {
	m.lastParams = params
	return m.result, m.err
}

func TestNewReviewer_RequiresGenerator(t *testing.T) {
	_, err := NewReviewer(nil, nil)
	require.Error(t, err)
}

func TestReviewer_Review_MapsModelOutput(t *testing.T) {
	reviewer, err := NewReviewer(&mockGenerator{
		result: map[string]any{
			"summary": "done",
			"findings": []any{
				map[string]any{
					"filePath":   "a.go",
					"startLine":  9,
					"endLine":    9,
					"severity":   "MAJOR",
					"title":      "Nil risk",
					"detail":     "Potential nil dereference",
					"suggestion": "Check nil",
				},
			},
		},
	}, nil)
	require.NoError(t, err)

	result, err := reviewer.Review(context.Background(), usecase.LLMReviewPayload{
		Input: domain.ReviewInput{
			ChangedFiles: []domain.ChangedFile{
				{
					Path:        "a.go",
					DiffSnippet: "@@ -9,1 +9,1 @@\n-old\n+new",
				},
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "done", result.Summary)
	require.Len(t, result.Findings, 1)
	require.Equal(t, domain.FindingSeverityMajor, result.Findings[0].Severity)
	require.Equal(t, 9, result.Findings[0].StartLine)
	require.Equal(t, 9, result.Findings[0].EndLine)
}

func TestReviewer_Review_SummaryOnly(t *testing.T) {
	reviewer, err := NewReviewer(&mockGenerator{
		result: map[string]any{
			"summary": "done",
		},
	}, nil)
	require.NoError(t, err)

	result, err := reviewer.Review(context.Background(), usecase.LLMReviewPayload{})
	require.NoError(t, err)
	require.Equal(t, "done", result.Summary)
	require.Empty(t, result.Findings)
}

func TestReviewer_Review_InvalidFindingsShape(t *testing.T) {
	reviewer, err := NewReviewer(&mockGenerator{
		result: map[string]any{
			"summary":  "done",
			"findings": "invalid",
		},
	}, nil)
	require.NoError(t, err)

	_, err = reviewer.Review(context.Background(), usecase.LLMReviewPayload{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid review model output")
}

func TestReviewer_Review_RejectsMissingStartLine(t *testing.T) {
	reviewer, err := NewReviewer(&mockGenerator{
		result: map[string]any{
			"summary": "done",
			"findings": []any{
				map[string]any{
					"filePath":   "a.go",
					"endLine":    9,
					"severity":   "MAJOR",
					"title":      "Nil risk",
					"detail":     "Potential nil dereference",
					"suggestion": "Check nil",
				},
			},
		},
	}, nil)
	require.NoError(t, err)

	_, err = reviewer.Review(context.Background(), usecase.LLMReviewPayload{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid finding range")
}

func TestReviewer_Review_RejectsMissingEndLine(t *testing.T) {
	reviewer, err := NewReviewer(&mockGenerator{
		result: map[string]any{
			"summary": "done",
			"findings": []any{
				map[string]any{
					"filePath":   "a.go",
					"startLine":  9,
					"severity":   "MAJOR",
					"title":      "Nil risk",
					"detail":     "Potential nil dereference",
					"suggestion": "Check nil",
				},
			},
		},
	}, nil)
	require.NoError(t, err)

	_, err = reviewer.Review(context.Background(), usecase.LLMReviewPayload{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid finding range")
}

func TestReviewer_Review_RejectsInvalidRange(t *testing.T) {
	reviewer, err := NewReviewer(&mockGenerator{
		result: map[string]any{
			"summary": "done",
			"findings": []any{
				map[string]any{
					"filePath":   "a.go",
					"startLine":  11,
					"endLine":    9,
					"severity":   "MAJOR",
					"title":      "Nil risk",
					"detail":     "Potential nil dereference",
					"suggestion": "Check nil",
				},
			},
		},
	}, nil)
	require.NoError(t, err)

	_, err = reviewer.Review(context.Background(), usecase.LLMReviewPayload{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid finding range")
}

func TestReviewer_Review_RendersSystemPromptFromSingleRulePackString(t *testing.T) {
	generator := &mockGenerator{
		result: map[string]any{
			"summary": "done",
		},
	}
	reviewer, err := NewReviewer(generator, nil)
	require.NoError(t, err)

	_, err = reviewer.Review(context.Background(), usecase.LLMReviewPayload{
		RulePack: usecase.RulePack{
			Instructions: []string{
				"first instruction",
				"second instruction",
			},
		},
	})
	require.NoError(t, err)
	require.Contains(t, generator.lastParams.SystemPrompt, "first instruction\n\nsecond instruction")
	require.Contains(t, generator.lastParams.SystemPrompt, "{\"summary\": string, \"findings\": []}")
}

func TestReviewer_Review_SetsResponseSchema(t *testing.T) {
	generator := &mockGenerator{
		result: map[string]any{
			"summary":  "done",
			"findings": []any{},
		},
	}
	reviewer, err := NewReviewer(generator, nil)
	require.NoError(t, err)

	_, err = reviewer.Review(context.Background(), usecase.LLMReviewPayload{})
	require.NoError(t, err)

	require.NotNil(t, generator.lastParams.ResponseSchema)
	require.Equal(t, "object", generator.lastParams.ResponseSchema["type"])

	propertiesRaw, ok := generator.lastParams.ResponseSchema["properties"]
	require.True(t, ok)
	properties, ok := propertiesRaw.(map[string]any)
	require.True(t, ok)
	require.Contains(t, properties, "summary")
	require.Contains(t, properties, "findings")
}

func TestReviewer_Review_RendersUserPromptInNaturalLanguage(t *testing.T) {
	generator := &mockGenerator{
		result: map[string]any{
			"summary": "done",
		},
	}
	reviewer, err := NewReviewer(generator, nil)
	require.NoError(t, err)

	_, err = reviewer.Review(context.Background(), usecase.LLMReviewPayload{
		Input: domain.ReviewInput{
			Title:       "Improve validation",
			Description: "Add nil checks and error handling.",
			Language:    "English",
			ChangedFiles: []domain.ChangedFile{
				{
					Path:        "service/a.go",
					DiffSnippet: "@@ -10,2 +10,4 @@\n+if input == nil { return err }",
				},
			},
		},
	})
	require.NoError(t, err)
	require.Len(t, generator.lastParams.Messages, 1)
	require.Equal(t, "user", generator.lastParams.Messages[0].Role)
	require.Contains(t, generator.lastParams.Messages[0].Content, "Review the following code changes")
	require.Contains(t, generator.lastParams.Messages[0].Content, "File: service/a.go")
	require.Contains(t, generator.lastParams.Messages[0].Content, "+if input == nil { return err }")
	require.NotContains(t, generator.lastParams.Messages[0].Content, "\"input\"")
}

func TestReviewer_Review_SplitsFindingByChangedSegments(t *testing.T) {
	reviewer, err := NewReviewer(&mockGenerator{
		result: map[string]any{
			"summary": "done",
			"findings": []any{
				map[string]any{
					"filePath":   "a.go",
					"startLine":  10,
					"endLine":    20,
					"severity":   "MAJOR",
					"title":      "Range issue",
					"detail":     "detail",
					"suggestion": "fix",
				},
			},
		},
	}, nil)
	require.NoError(t, err)

	result, err := reviewer.Review(context.Background(), usecase.LLMReviewPayload{
		Input: domain.ReviewInput{
			ChangedFiles: []domain.ChangedFile{
				{
					Path: "a.go",
					DiffSnippet: strings.Join([]string{
						"@@ -10,2 +10,2 @@",
						"+x",
						"+y",
						"@@ -15,2 +15,2 @@",
						"+a",
						"+b",
					}, "\n"),
				},
			},
		},
	})
	require.NoError(t, err)
	require.Len(t, result.Findings, 2)
	require.Equal(t, 10, result.Findings[0].StartLine)
	require.Equal(t, 11, result.Findings[0].EndLine)
	require.Equal(t, 15, result.Findings[1].StartLine)
	require.Equal(t, 16, result.Findings[1].EndLine)
	require.Equal(t, "Range issue", result.Findings[0].Title)
	require.Equal(t, "Range issue", result.Findings[1].Title)
}

func TestReviewer_Review_DropsWhenNoOverlap(t *testing.T) {
	reviewer, err := NewReviewer(&mockGenerator{
		result: map[string]any{
			"summary": "done",
			"findings": []any{
				map[string]any{
					"filePath":   "a.go",
					"startLine":  30,
					"endLine":    35,
					"severity":   "MAJOR",
					"title":      "No overlap",
					"detail":     "detail",
					"suggestion": "fix",
				},
			},
		},
	}, nil)
	require.NoError(t, err)

	result, err := reviewer.Review(context.Background(), usecase.LLMReviewPayload{
		Input: domain.ReviewInput{
			ChangedFiles: []domain.ChangedFile{
				{
					Path:        "a.go",
					DiffSnippet: "@@ -10,1 +10,1 @@\n-old\n+new",
				},
			},
		},
	})
	require.NoError(t, err)
	require.Empty(t, result.Findings)
}

func TestReviewer_Review_CapsSplitToThree(t *testing.T) {
	reviewer, err := NewReviewer(&mockGenerator{
		result: map[string]any{
			"summary": "done",
			"findings": []any{
				map[string]any{
					"filePath":   "a.go",
					"startLine":  1,
					"endLine":    100,
					"severity":   "MINOR",
					"title":      "Many segments",
					"detail":     "detail",
					"suggestion": "fix",
				},
			},
		},
	}, nil)
	require.NoError(t, err)

	result, err := reviewer.Review(context.Background(), usecase.LLMReviewPayload{
		Input: domain.ReviewInput{
			ChangedFiles: []domain.ChangedFile{
				{
					Path: "a.go",
					DiffSnippet: strings.Join([]string{
						"@@ -1,1 +1,1 @@",
						"+a",
						"@@ -10,1 +10,1 @@",
						"+b",
						"@@ -20,1 +20,1 @@",
						"+c",
						"@@ -30,1 +30,1 @@",
						"+d",
					}, "\n"),
				},
			},
		},
	})
	require.NoError(t, err)
	require.Len(t, result.Findings, 3)
	require.Equal(t, 1, result.Findings[0].StartLine)
	require.Equal(t, 10, result.Findings[1].StartLine)
	require.Equal(t, 20, result.Findings[2].StartLine)
}

func TestReviewer_Review_DropsFindingsWhenDiffInvalid(t *testing.T) {
	reviewer, err := NewReviewer(&mockGenerator{
		result: map[string]any{
			"summary": "done",
			"findings": []any{
				map[string]any{
					"filePath":   "a.go",
					"startLine":  1,
					"endLine":    1,
					"severity":   "MINOR",
					"title":      "Invalid diff",
					"detail":     "detail",
					"suggestion": "fix",
				},
			},
		},
	}, nil)
	require.NoError(t, err)

	result, err := reviewer.Review(context.Background(), usecase.LLMReviewPayload{
		Input: domain.ReviewInput{
			ChangedFiles: []domain.ChangedFile{
				{
					Path:        "a.go",
					DiffSnippet: "not a unified diff",
				},
			},
		},
	})
	require.NoError(t, err)
	require.Empty(t, result.Findings)
}

func TestReviewer_Review_ReturnsErrorWhenSystemPromptTemplateInvalid(t *testing.T) {
	originalTemplate := reviewSystemPromptTemplateRaw
	reviewSystemPromptTemplateRaw = "{{ .RulePackText "
	defer func() {
		reviewSystemPromptTemplateRaw = originalTemplate
	}()

	reviewer, err := NewReviewer(&mockGenerator{
		result: map[string]any{
			"summary": "done",
		},
	}, nil)
	require.NoError(t, err)

	_, err = reviewer.Review(context.Background(), usecase.LLMReviewPayload{})
	require.Error(t, err)
}

func TestReviewer_Review_ReturnsErrorWhenUserPromptTemplateInvalid(t *testing.T) {
	originalTemplate := reviewUserPromptTemplateRaw
	reviewUserPromptTemplateRaw = "{{ .Language "
	defer func() {
		reviewUserPromptTemplateRaw = originalTemplate
	}()

	reviewer, err := NewReviewer(&mockGenerator{
		result: map[string]any{
			"summary": "done",
		},
	}, nil)
	require.NoError(t, err)

	_, err = reviewer.Review(context.Background(), usecase.LLMReviewPayload{})
	require.Error(t, err)
}

func TestReviewer_GroupFindings_MapsModelOutput(t *testing.T) {
	generator := &mockGenerator{
		result: map[string]any{
			"groups": []any{
				map[string]any{
					"groupId":     "g1",
					"rationale":   "same file",
					"findingKeys": []any{"a.go:9:9:Nil risk"},
				},
			},
		},
	}
	reviewer, err := NewReviewer(generator, nil)
	require.NoError(t, err)

	result, err := reviewer.GroupFindings(context.Background(), usecase.LLMSuggestionGroupingPayload{
		MaxGroupSize: 5,
		Candidates: []usecase.SuggestionFindingCandidate{
			{
				Key: "a.go:9:9:Nil risk",
				Finding: domain.Finding{
					FilePath:  "a.go",
					StartLine: 9,
					EndLine:   9,
					Severity:  domain.FindingSeverityMajor,
					Title:     "Nil risk",
					Detail:    "Potential nil dereference",
				},
				DiffSnippet: "@@ -9 +9 @@",
			},
		},
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)
	require.Equal(t, "g1", result.Groups[0].GroupID)
	require.Contains(t, generator.lastParams.Messages[0].Content, "Relevant diff/context")
}

func TestReviewer_GenerateSuggestedChanges_MapsSuggestedChangesAndAcceptsDeleteWithEmptyReplacement(t *testing.T) {
	generator := &mockGenerator{
		result: map[string]any{
			"suggestions": []any{
				map[string]any{
					"findingKey":  "a.go:9:9:Nil risk",
					"kind":        "REPLACE",
					"replacement": "if err != nil { return err }",
					"reason":      "Prevents nil dereference on error handling path.",
				},
				map[string]any{
					"findingKey":  "a.go:12:13:Dead code",
					"kind":        "DELETE",
					"replacement": "",
					"reason":      "Removes unreachable logic that never executes.",
				},
			},
		},
	}
	reviewer, err := NewReviewer(generator, nil)
	require.NoError(t, err)

	result, err := reviewer.GenerateSuggestedChanges(context.Background(), usecase.LLMSuggestedChangePayload{
		Group: usecase.SuggestionFindingGroup{GroupID: "g1", Rationale: "same file"},
		GroupDiffs: []usecase.GroupFileDiffContext{
			{
				FilePath:    "a.go",
				DiffSnippet: "@@ -9,3 +9,3 @@",
			},
		},
		Candidates: []usecase.SuggestionFindingCandidate{
			{
				Key: "a.go:9:9:Nil risk",
				Finding: domain.Finding{
					FilePath:  "a.go",
					StartLine: 9,
					EndLine:   9,
					Severity:  domain.FindingSeverityMajor,
					Title:     "Nil risk",
					Detail:    "Potential nil dereference",
				},
				DiffSnippet: "@@ -9 +9 @@",
			},
			{
				Key: "a.go:12:13:Dead code",
				Finding: domain.Finding{
					FilePath:  "a.go",
					StartLine: 12,
					EndLine:   13,
					Severity:  domain.FindingSeverityMajor,
					Title:     "Dead code",
					Detail:    "Unreachable block",
				},
				DiffSnippet: "@@ -12,2 +12,0 @@",
			},
		},
	})
	require.NoError(t, err)
	require.Len(t, result.Suggestions, 2)
	require.Equal(t, domain.SuggestedChangeKindReplace, result.Suggestions[0].SuggestedChange.Kind)
	require.Equal(t, domain.SuggestedChangeKindDelete, result.Suggestions[1].SuggestedChange.Kind)
	require.NotEmpty(t, result.Suggestions[0].SuggestedChange.Reason)
	require.Contains(t, generator.lastParams.Messages[0].Content, "Group file diffs")
	require.Contains(t, generator.lastParams.Messages[0].Content, "File: a.go")
	require.Contains(t, generator.lastParams.Messages[0].Content, "@@ -9,3 +9,3 @@")
}

func TestReviewer_GenerateSuggestedChanges_DropsDeleteWithNonEmptyReplacement(t *testing.T) {
	reviewer, err := NewReviewer(&mockGenerator{
		result: map[string]any{
			"suggestions": []any{
				map[string]any{
					"findingKey":  "a.go:12:13:Dead code",
					"kind":        "DELETE",
					"replacement": "unexpected",
					"reason":      "Dead code should be removed.",
				},
			},
		},
	}, nil)
	require.NoError(t, err)

	result, err := reviewer.GenerateSuggestedChanges(context.Background(), usecase.LLMSuggestedChangePayload{
		Group: usecase.SuggestionFindingGroup{GroupID: "g1"},
		Candidates: []usecase.SuggestionFindingCandidate{
			{Key: "a.go:12:13:Dead code"},
		},
	})
	require.NoError(t, err)
	require.Empty(t, result.Suggestions)
}

func TestReviewer_GenerateSuggestedChanges_DropsSuggestionWithEmptyReason(t *testing.T) {
	reviewer, err := NewReviewer(&mockGenerator{
		result: map[string]any{
			"suggestions": []any{
				map[string]any{
					"findingKey":  "a.go:9:9:Nil risk",
					"kind":        "REPLACE",
					"replacement": "if err != nil { return err }",
					"reason":      "   ",
				},
			},
		},
	}, nil)
	require.NoError(t, err)

	result, err := reviewer.GenerateSuggestedChanges(context.Background(), usecase.LLMSuggestedChangePayload{
		Group: usecase.SuggestionFindingGroup{GroupID: "g1"},
		Candidates: []usecase.SuggestionFindingCandidate{
			{Key: "a.go:9:9:Nil risk"},
		},
	})
	require.NoError(t, err)
	require.Empty(t, result.Suggestions)
}
