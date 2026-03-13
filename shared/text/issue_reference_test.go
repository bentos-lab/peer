package text

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractIssueReferences(t *testing.T) {
	input := "Fixes #12 and relates to org/other#34. See https://github.com/org/repo/issues/56"
	refs := ExtractIssueReferences(input, "org/repo")
	require.Len(t, refs, 3)
	require.Equal(t, "org/repo", refs[0].Repository)
	require.Equal(t, 12, refs[0].Number)
	require.Equal(t, "org/other", refs[1].Repository)
	require.Equal(t, 34, refs[1].Number)
	require.Equal(t, "org/repo", refs[2].Repository)
	require.Equal(t, 56, refs[2].Number)
}

func TestExtractIssueReferencesSkipsWhenNoDefaultRepo(t *testing.T) {
	input := "Fixes #12"
	refs := ExtractIssueReferences(input, "")
	require.Empty(t, refs)
}

func TestExtractIssueReferencesDedupes(t *testing.T) {
	input := "Fixes #12 and https://github.com/org/repo/issues/12"
	refs := ExtractIssueReferences(input, "org/repo")
	require.Len(t, refs, 1)
	require.Equal(t, "org/repo", refs[0].Repository)
	require.Equal(t, 12, refs[0].Number)
}
