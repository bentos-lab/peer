package text

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSingleLineCollapsesWhitespace(t *testing.T) {
	input := "Title\n\nBody\twith\tspaces"
	require.Equal(t, "Title Body with spaces", SingleLine(input))
}

func TestSingleLineCollapsesBlankLines(t *testing.T) {
	input := "Line one\n\n\nLine two"
	require.Equal(t, "Line one Line two", SingleLine(input))
}

func TestSingleLineEmptyInput(t *testing.T) {
	require.Equal(t, "", SingleLine(""))
	require.Equal(t, "", SingleLine("   \n\t"))
}
