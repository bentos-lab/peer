package usecase

import (
	"testing"

	"bentos-backend/domain"
	"github.com/stretchr/testify/require"
)

func TestBuildMessages_GroupsByFileAndAddsSummary(t *testing.T) {
	findings := []domain.Finding{
		{
			FilePath:   "a.go",
			Line:       10,
			Severity:   domain.FindingSeverityMajor,
			Title:      "Nil risk",
			Detail:     "Potential nil pointer dereference.",
			Suggestion: "Check value before use.",
		},
		{
			FilePath: "b.go",
			Line:     5,
			Severity: domain.FindingSeverityMinor,
			Title:    "Complex branch",
			Detail:   "Nested branch is hard to read.",
		},
		{
			FilePath: "a.go",
			Line:     20,
			Severity: domain.FindingSeverityNit,
			Title:    "Naming",
			Detail:   "Variable name can be clearer.",
		},
	}

	messages := BuildMessages(findings, "")
	require.Len(t, messages, 3)
	require.Equal(t, domain.ReviewMessageTypeFileGroup, messages[0].Type)
	require.Equal(t, "a.go", messages[0].FilePath)
	require.Equal(t, domain.ReviewMessageTypeFileGroup, messages[1].Type)
	require.Equal(t, "b.go", messages[1].FilePath)
	require.Equal(t, domain.ReviewMessageTypeSummary, messages[2].Type)
	require.Contains(t, messages[2].Body, "Found 3 potential issue(s)")
}

func TestBuildMessages_UsesLLMSummaryWhenProvided(t *testing.T) {
	messages := BuildMessages(nil, "Custom summary")
	require.Len(t, messages, 1)
	require.Equal(t, domain.ReviewMessageTypeSummary, messages[0].Type)
	require.Equal(t, "Custom summary", messages[0].Body)
}
