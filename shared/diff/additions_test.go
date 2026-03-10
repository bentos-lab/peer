package diff

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractAddedBlocksSingleHunk(t *testing.T) {
	diff := "@@ -1,2 +1,3 @@\n line1\n-line2\n+line2b\n+line3\n"
	blocks := ExtractAddedBlocks(diff)
	require.Len(t, blocks, 1)
	require.Equal(t, 2, blocks[0].StartLine)
	require.Equal(t, 3, blocks[0].EndLine)
	require.Equal(t, "line2b\nline3", blocks[0].Content)
}

func TestExtractAddedBlocksMultipleHunks(t *testing.T) {
	diff := "@@ -1,1 +1,2 @@\n line1\n+line2\n@@ -10,1 +11,2 @@\n line10\n+line11\n+line12\n"
	blocks := ExtractAddedBlocks(diff)
	require.Len(t, blocks, 2)
	require.Equal(t, 2, blocks[0].StartLine)
	require.Equal(t, 2, blocks[0].EndLine)
	require.Equal(t, "line2", blocks[0].Content)
	require.Equal(t, 13, blocks[1].EndLine)
	require.Equal(t, "line11\nline12", blocks[1].Content)
}

func TestExtractAddedBlocksSkipsHeaders(t *testing.T) {
	diff := "diff --git a/a.go b/a.go\nindex 123..456 100644\n--- a/a.go\n+++ b/a.go\n@@ -1,1 +1,2 @@\n line1\n+line2\n"
	blocks := ExtractAddedBlocks(diff)
	require.Len(t, blocks, 1)
	require.Equal(t, 2, blocks[0].StartLine)
	require.Equal(t, "line2", blocks[0].Content)
}
