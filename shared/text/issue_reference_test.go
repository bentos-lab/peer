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

func TestExtractGitLabIssueReferences(t *testing.T) {
	input := "Relates to https://gitlab.com/group/subgroup/project/-/issues/42 and group/subgroup/project#7 plus #9."
	refs := ExtractGitLabIssueReferences(input, "group/subgroup/project")
	require.Len(t, refs, 3)
	require.Equal(t, "group/subgroup/project", refs[0].Repository)
	require.Equal(t, 42, refs[0].Number)
	require.Equal(t, "group/subgroup/project", refs[1].Repository)
	require.Equal(t, 7, refs[1].Number)
	require.Equal(t, "group/subgroup/project", refs[2].Repository)
	require.Equal(t, 9, refs[2].Number)
}

func TestExtractGitLabIssueReferencesSkipsWhenNoDefaultRepo(t *testing.T) {
	input := "Fixes #12"
	refs := ExtractGitLabIssueReferences(input, "")
	require.Empty(t, refs)
}
