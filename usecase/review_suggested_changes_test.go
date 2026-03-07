package usecase

import (
	"testing"

	"bentos-backend/domain"
	"github.com/stretchr/testify/require"
)

func TestFilterSuggestionCandidates_KeepsValidHighSeverityUniqueCandidates(t *testing.T) {
	input := domain.ReviewInput{
		ChangedFiles: []domain.ChangedFile{
			{Path: "a.go", DiffSnippet: "@@ -1 +1 @@\n-old\n+new"},
		},
	}
	findings := []domain.Finding{
		{
			FilePath:  "a.go",
			StartLine: 10,
			EndLine:   10,
			Severity:  domain.FindingSeverityMajor,
			Title:     "A",
			Detail:    "detail",
		},
		{
			FilePath:  "a.go",
			StartLine: 10,
			EndLine:   10,
			Severity:  domain.FindingSeverityMajor,
			Title:     "A",
			Detail:    "duplicate",
		},
		{
			FilePath:  "a.go",
			StartLine: 11,
			EndLine:   11,
			Severity:  domain.FindingSeverityNit,
			Title:     "N",
			Detail:    "nit",
		},
	}

	candidates := filterSuggestionCandidates(input, findings, normalizeSuggestedChangesConfig(SuggestedChangesConfig{
		MinSeverity:   domain.FindingSeverityMajor,
		MaxCandidates: 50,
	}))
	require.Len(t, candidates, 1)
	require.Equal(t, "finding-1", candidates[0].Key)
	require.Contains(t, candidates[0].DiffSnippet, "@@ -1 +1 @@")
}

func TestFilterSuggestionCandidates_UsesStableOpaqueKeys(t *testing.T) {
	input := domain.ReviewInput{
		ChangedFiles: []domain.ChangedFile{
			{Path: "a.go", DiffSnippet: "@@ -1 +1 @@\n-old\n+new"},
			{Path: "b.go", DiffSnippet: "@@ -2 +2 @@\n-old2\n+new2"},
		},
	}
	findingsA := []domain.Finding{
		{FilePath: "a.go", StartLine: 10, EndLine: 10, Severity: domain.FindingSeverityMajor, Title: "First title", Detail: "detail"},
		{FilePath: "b.go", StartLine: 20, EndLine: 20, Severity: domain.FindingSeverityMajor, Title: "Second title", Detail: "detail"},
	}
	findingsB := []domain.Finding{
		{FilePath: "a.go", StartLine: 10, EndLine: 10, Severity: domain.FindingSeverityMajor, Title: "Renamed first", Detail: "detail"},
		{FilePath: "b.go", StartLine: 20, EndLine: 20, Severity: domain.FindingSeverityMajor, Title: "Renamed second", Detail: "detail"},
	}

	candidatesA := filterSuggestionCandidates(input, findingsA, normalizeSuggestedChangesConfig(SuggestedChangesConfig{
		MinSeverity:   domain.FindingSeverityMajor,
		MaxCandidates: 50,
	}))
	candidatesB := filterSuggestionCandidates(input, findingsB, normalizeSuggestedChangesConfig(SuggestedChangesConfig{
		MinSeverity:   domain.FindingSeverityMajor,
		MaxCandidates: 50,
	}))

	require.Len(t, candidatesA, 2)
	require.Len(t, candidatesB, 2)
	require.Equal(t, "finding-1", candidatesA[0].Key)
	require.Equal(t, "finding-2", candidatesA[1].Key)
	require.Equal(t, candidatesA[0].Key, candidatesB[0].Key)
	require.Equal(t, candidatesA[1].Key, candidatesB[1].Key)
}

func TestIsValidGrouping_RejectsDuplicateKeysAcrossGroups(t *testing.T) {
	candidates := []SuggestionFindingCandidate{
		{Key: "k1"},
		{Key: "k2"},
	}
	result := LLMSuggestionGroupingResult{
		Groups: []SuggestionFindingGroup{
			{GroupID: "g1", FindingKeys: []string{"k1", "k2"}},
			{GroupID: "g2", FindingKeys: []string{"k2"}},
		},
	}
	require.False(t, isValidGrouping(result, candidates, 5))
}

func TestDeterministicGroups_RespectsMaxGroupSize(t *testing.T) {
	candidates := []SuggestionFindingCandidate{
		{Key: "k1", Finding: domain.Finding{FilePath: "a.go", StartLine: 1, Title: "A"}},
		{Key: "k2", Finding: domain.Finding{FilePath: "a.go", StartLine: 2, Title: "B"}},
		{Key: "k3", Finding: domain.Finding{FilePath: "a.go", StartLine: 3, Title: "C"}},
	}

	groups := deterministicGroups(candidates, 2)
	require.Len(t, groups, 2)
	require.Len(t, groups[0].FindingKeys, 2)
	require.Len(t, groups[1].FindingKeys, 1)
}

func TestBuildGroupDiffs_UsesOnlyGroupFilesAndFallsBackToContent(t *testing.T) {
	input := domain.ReviewInput{
		ChangedFiles: []domain.ChangedFile{
			{Path: "a.go", DiffSnippet: "@@ -1 +1 @@\n-old\n+new"},
			{Path: "b.go", Content: "package b\n\nfunc x() {}"},
			{Path: "c.go", DiffSnippet: "@@ -3 +3 @@\n-oldc\n+newc"},
		},
	}
	candidates := []SuggestionFindingCandidate{
		{Finding: domain.Finding{FilePath: "b.go"}},
		{Finding: domain.Finding{FilePath: "a.go"}},
	}

	groupDiffs := buildGroupDiffs(input, candidates)
	require.Len(t, groupDiffs, 2)
	require.Equal(t, "a.go", groupDiffs[0].FilePath)
	require.Equal(t, "b.go", groupDiffs[1].FilePath)
	require.Contains(t, groupDiffs[0].DiffSnippet, "@@ -1 +1 @@")
	require.Contains(t, groupDiffs[1].DiffSnippet, "package b")
}
