package codingagent

import (
	"strings"

	"github.com/bentos-lab/peer/domain"
)

func normalizeSuggestedChange(change *domain.SuggestedChange) *domain.SuggestedChange {
	if change == nil {
		return nil
	}
	change.Kind = domain.SuggestedChangeKindEnum(strings.TrimSpace(string(change.Kind)))
	change.Reason = strings.TrimSpace(change.Reason)
	if change.StartLine <= 0 || change.EndLine <= 0 || change.StartLine > change.EndLine {
		return nil
	}
	if change.Kind != domain.SuggestedChangeKindReplace && change.Kind != domain.SuggestedChangeKindDelete {
		return nil
	}
	if change.Reason == "" {
		return nil
	}
	if change.Kind == domain.SuggestedChangeKindDelete {
		if strings.TrimSpace(change.Replacement) != "" {
			return nil
		}
		change.Replacement = ""
		return change
	}
	if strings.TrimSpace(change.Replacement) == "" {
		return nil
	}
	return change
}

func resolveLanguage(language string) string {
	trimmed := strings.TrimSpace(language)
	if trimmed == "" {
		return "English"
	}
	return trimmed
}
