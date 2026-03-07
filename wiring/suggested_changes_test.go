package wiring

import (
	"context"
	"testing"

	"bentos-backend/config"
	"bentos-backend/domain"
	"bentos-backend/usecase"
	"github.com/stretchr/testify/require"
)

type suggestionCapableReviewer struct{}

func (suggestionCapableReviewer) Review(_ context.Context, _ usecase.LLMReviewPayload) (usecase.LLMReviewResult, error) {
	return usecase.LLMReviewResult{}, nil
}

func (suggestionCapableReviewer) GroupFindings(_ context.Context, _ usecase.LLMSuggestionGroupingPayload) (usecase.LLMSuggestionGroupingResult, error) {
	return usecase.LLMSuggestionGroupingResult{}, nil
}

func (suggestionCapableReviewer) GenerateSuggestedChanges(_ context.Context, _ usecase.LLMSuggestedChangePayload) (usecase.LLMSuggestedChangeResult, error) {
	return usecase.LLMSuggestedChangeResult{}, nil
}

func TestReviewUseCaseOptionsFromConfig_DisabledReturnsNoOptions(t *testing.T) {
	options, err := reviewUseCaseOptionsFromConfig(config.Config{}, suggestionCapableReviewer{})
	require.NoError(t, err)
	require.Empty(t, options)
}

func TestReviewUseCaseOptionsFromConfig_RejectsInvalidSeverity(t *testing.T) {
	_, err := reviewUseCaseOptionsFromConfig(config.Config{
		SuggestedChanges: config.SuggestedChangesConfig{
			Enabled:     true,
			MinSeverity: "INVALID",
		},
	}, suggestionCapableReviewer{})
	require.Error(t, err)
}

func TestParseFindingSeverity_DefaultsToMajor(t *testing.T) {
	severity, err := parseFindingSeverity("")
	require.NoError(t, err)
	require.Equal(t, domain.FindingSeverityMajor, severity)
}
