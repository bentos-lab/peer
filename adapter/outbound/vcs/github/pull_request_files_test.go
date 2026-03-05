package github

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMapPullRequestFilesToChangedFiles(t *testing.T) {
	files := []pullRequestFile{
		{Filename: "a.go", Patch: "@@ -1 +1 @@\n-old\n+new"},
		{Filename: "b.go", Patch: "   "},
	}

	mapped := mapPullRequestFilesToChangedFiles(files)
	require.Len(t, mapped, 1)
	require.Equal(t, "a.go", mapped[0].Path)
	require.Contains(t, mapped[0].DiffSnippet, "+new")
}

func TestParsePullRequestFilesUnexpectedPayload(t *testing.T) {
	_, err := parsePullRequestFiles([]byte(`{"foo":"bar"}`))
	require.Error(t, err)
	require.Contains(t, err.Error(), "unexpected pull request files payload")
}
