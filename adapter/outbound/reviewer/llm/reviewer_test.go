package llm

import (
	"context"
	"errors"
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

func TestReviewer_ReviewDiff_MapsModelOutput(t *testing.T) {
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

	result, err := reviewer.ReviewDiff(context.Background(), usecase.LLMReviewPayload{})
	require.NoError(t, err)
	require.Equal(t, "done", result.Summary)
	require.Len(t, result.Findings, 1)
	require.Equal(t, domain.FindingSeverityMajor, result.Findings[0].Severity)
	require.Equal(t, 9, result.Findings[0].StartLine)
	require.Equal(t, 9, result.Findings[0].EndLine)
}

func TestReviewer_ReviewDiff_SummaryOnly(t *testing.T) {
	reviewer, err := NewReviewer(&mockGenerator{
		result: map[string]any{
			"summary": "done",
		},
	}, nil)
	require.NoError(t, err)

	result, err := reviewer.ReviewDiff(context.Background(), usecase.LLMReviewPayload{})
	require.NoError(t, err)
	require.Equal(t, "done", result.Summary)
	require.Empty(t, result.Findings)
}

func TestReviewer_ReviewDiff_InvalidFindingsShape(t *testing.T) {
	reviewer, err := NewReviewer(&mockGenerator{
		result: map[string]any{
			"summary":  "done",
			"findings": "invalid",
		},
	}, nil)
	require.NoError(t, err)

	_, err = reviewer.ReviewDiff(context.Background(), usecase.LLMReviewPayload{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid review model output")
}

func TestReviewer_ReviewDiff_RejectsMissingStartLine(t *testing.T) {
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

	_, err = reviewer.ReviewDiff(context.Background(), usecase.LLMReviewPayload{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid finding range")
}

func TestReviewer_ReviewDiff_RejectsMissingEndLine(t *testing.T) {
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

	_, err = reviewer.ReviewDiff(context.Background(), usecase.LLMReviewPayload{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid finding range")
}

func TestReviewer_ReviewDiff_RejectsInvalidRange(t *testing.T) {
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

	_, err = reviewer.ReviewDiff(context.Background(), usecase.LLMReviewPayload{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid finding range")
}

func TestReviewer_ReviewDiff_RendersSystemPromptFromSingleRulePackString(t *testing.T) {
	generator := &mockGenerator{
		result: map[string]any{
			"summary": "done",
		},
	}
	reviewer, err := NewReviewer(generator, nil)
	require.NoError(t, err)

	_, err = reviewer.ReviewDiff(context.Background(), usecase.LLMReviewPayload{
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

func TestReviewer_ReviewDiff_SetsResponseSchema(t *testing.T) {
	generator := &mockGenerator{
		result: map[string]any{
			"summary":  "done",
			"findings": []any{},
		},
	}
	reviewer, err := NewReviewer(generator, nil)
	require.NoError(t, err)

	_, err = reviewer.ReviewDiff(context.Background(), usecase.LLMReviewPayload{})
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

func TestReviewer_ReviewDiff_RendersUserPromptInNaturalLanguage(t *testing.T) {
	generator := &mockGenerator{
		result: map[string]any{
			"summary": "done",
		},
	}
	reviewer, err := NewReviewer(generator, nil)
	require.NoError(t, err)

	_, err = reviewer.ReviewDiff(context.Background(), usecase.LLMReviewPayload{
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

func TestReviewer_ReviewDiff_ReturnsErrorWhenSystemPromptTemplateInvalid(t *testing.T) {
	originalTemplate := systemPromptTemplateRaw
	systemPromptTemplateRaw = "{{ .RulePackText "
	defer func() {
		systemPromptTemplateRaw = originalTemplate
	}()

	reviewer, err := NewReviewer(&mockGenerator{
		result: map[string]any{
			"summary": "done",
		},
	}, nil)
	require.NoError(t, err)

	_, err = reviewer.ReviewDiff(context.Background(), usecase.LLMReviewPayload{})
	require.Error(t, err)
}

func TestReviewer_ReviewDiff_ReturnsErrorWhenUserPromptTemplateInvalid(t *testing.T) {
	originalTemplate := userPromptTemplateRaw
	userPromptTemplateRaw = "{{ .Language "
	defer func() {
		userPromptTemplateRaw = originalTemplate
	}()

	reviewer, err := NewReviewer(&mockGenerator{
		result: map[string]any{
			"summary": "done",
		},
	}, nil)
	require.NoError(t, err)

	_, err = reviewer.ReviewDiff(context.Background(), usecase.LLMReviewPayload{})
	require.Error(t, err)
}
