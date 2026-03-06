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

type mockOverviewGenerator struct {
	result     map[string]any
	err        error
	lastParams contracts.GenerateParams
}

func (m *mockOverviewGenerator) Generate(_ context.Context, _ contracts.GenerateParams) (string, error) {
	return "", errors.New("not used")
}

func (m *mockOverviewGenerator) GenerateJSON(_ context.Context, params contracts.GenerateParams) (map[string]any, error) {
	m.lastParams = params
	return m.result, m.err
}

func TestNewOverviewGenerator_RequiresGenerator(t *testing.T) {
	_, err := NewOverviewGenerator(nil, nil)
	require.Error(t, err)
}

func TestOverviewGenerator_GenerateOverview_MapsModelOutput(t *testing.T) {
	generator, err := NewOverviewGenerator(&mockOverviewGenerator{
		result: map[string]any{
			"categories": []any{
				map[string]any{
					"category": "Logic Updates",
					"summary":  "Changed request validation flow.",
				},
			},
			"walkthroughs": []any{
				map[string]any{
					"groupName": "Validation and handlers",
					"files":     []any{"a.go", "b.go"},
					"summary":   "Centralized input checks and adjusted handlers.",
				},
			},
		},
	}, nil)
	require.NoError(t, err)

	result, err := generator.GenerateOverview(context.Background(), usecase.LLMOverviewPayload{})
	require.NoError(t, err)
	require.Len(t, result.Categories, 1)
	require.Equal(t, domain.OverviewCategoryLogicUpdates, result.Categories[0].Category)
	require.Len(t, result.Walkthroughs, 1)
	require.Equal(t, "Validation and handlers", result.Walkthroughs[0].GroupName)
}

func TestOverviewGenerator_GenerateOverview_RejectsInvalidCategory(t *testing.T) {
	generator, err := NewOverviewGenerator(&mockOverviewGenerator{
		result: map[string]any{
			"categories": []any{
				map[string]any{
					"category": "Other",
					"summary":  "misc",
				},
			},
			"walkthroughs": []any{},
		},
	}, nil)
	require.NoError(t, err)

	_, err = generator.GenerateOverview(context.Background(), usecase.LLMOverviewPayload{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid overview category")
}

func TestOverviewGenerator_GenerateOverview_SetsResponseSchema(t *testing.T) {
	backend := &mockOverviewGenerator{
		result: map[string]any{
			"categories":   []any{},
			"walkthroughs": []any{},
		},
	}
	generator, err := NewOverviewGenerator(backend, nil)
	require.NoError(t, err)

	_, err = generator.GenerateOverview(context.Background(), usecase.LLMOverviewPayload{})
	require.NoError(t, err)

	require.NotNil(t, backend.lastParams.ResponseSchema)
	propertiesRaw, ok := backend.lastParams.ResponseSchema["properties"]
	require.True(t, ok)
	properties, ok := propertiesRaw.(map[string]any)
	require.True(t, ok)
	require.Contains(t, properties, "categories")
	require.Contains(t, properties, "walkthroughs")
}

func TestOverviewGenerator_GenerateOverview_RendersUserPrompt(t *testing.T) {
	backend := &mockOverviewGenerator{
		result: map[string]any{
			"categories":   []any{},
			"walkthroughs": []any{},
		},
	}
	generator, err := NewOverviewGenerator(backend, nil)
	require.NoError(t, err)

	_, err = generator.GenerateOverview(context.Background(), usecase.LLMOverviewPayload{
		Input: domain.OverviewInput{
			Title:       "Improve auth",
			Description: "Refactor middleware",
			ChangedFiles: []domain.ChangedFile{
				{Path: "auth.go", DiffSnippet: "@@\n+validate(user)"},
			},
		},
	})
	require.NoError(t, err)
	require.Len(t, backend.lastParams.Messages, 1)
	require.Contains(t, backend.lastParams.Messages[0].Content, "Change title: Improve auth")
	require.Contains(t, backend.lastParams.Messages[0].Content, "File: auth.go")
}
