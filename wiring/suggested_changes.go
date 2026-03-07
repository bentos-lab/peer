package wiring

import (
	"fmt"
	"strings"
	"time"

	"bentos-backend/config"
	"bentos-backend/domain"
	"bentos-backend/usecase"
)

func reviewUseCaseOptionsFromConfig(cfg config.Config, reviewer usecase.LLMReviewer) ([]usecase.ReviewUseCaseOption, error) {
	if !cfg.SuggestedChanges.Enabled {
		return nil, nil
	}

	grouping, ok := reviewer.(usecase.LLMSuggestionGrouping)
	if !ok {
		return nil, fmt.Errorf("configured reviewer does not support suggestion grouping")
	}
	generator, ok := reviewer.(usecase.LLMSuggestedChangeGenerator)
	if !ok {
		return nil, fmt.Errorf("configured reviewer does not support suggested change generation")
	}

	minSeverity, err := parseFindingSeverity(cfg.SuggestedChanges.MinSeverity)
	if err != nil {
		return nil, err
	}

	options := []usecase.ReviewUseCaseOption{
		usecase.WithSuggestedChanges(usecase.SuggestedChangesConfig{
			MinSeverity:     minSeverity,
			MaxCandidates:   cfg.SuggestedChanges.MaxCandidates,
			MaxGroupSize:    cfg.SuggestedChanges.MaxGroupSize,
			MaxWorkers:      cfg.SuggestedChanges.MaxWorkers,
			GroupTimeout:    time.Duration(cfg.SuggestedChanges.GroupTimeoutMS) * time.Millisecond,
			GenerateTimeout: time.Duration(cfg.SuggestedChanges.GenerateTimeoutMS) * time.Millisecond,
		}, grouping, generator),
	}

	return options, nil
}

func parseFindingSeverity(rawValue string) (domain.FindingSeverityEnum, error) {
	normalized := strings.TrimSpace(strings.ToUpper(rawValue))
	if normalized == "" {
		normalized = string(domain.FindingSeverityMajor)
	}
	severity := domain.FindingSeverityEnum(normalized)

	switch severity {
	case domain.FindingSeverityCritical, domain.FindingSeverityMajor, domain.FindingSeverityMinor, domain.FindingSeverityNit:
		return severity, nil
	default:
		return "", fmt.Errorf("invalid REVIEW_SUGGESTED_CHANGES_MIN_SEVERITY: %q", rawValue)
	}
}
