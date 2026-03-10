package text

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestContainsTriggerMatchesAtOrSlash(t *testing.T) {
	name := "AutoGitBot"

	require.True(t, ContainsTrigger("please /autogitbot do", name))
	require.True(t, ContainsTrigger("hi @AUTOGITBOT ok", name))
	require.True(t, ContainsTrigger("(@autogitbot) thanks", name))
	require.False(t, ContainsTrigger("just autogitbot", name))
	require.False(t, ContainsTrigger("/autogitbotty", name))
	require.False(t, ContainsTrigger("@autogitbotty", name))
}

func TestStripTriggerRemovesAllOccurrences(t *testing.T) {
	name := "autogitbot"
	input := "  @autogitbot  please\n /autogitbot  do  \n"

	require.Equal(t, "please\ndo", StripTrigger(input, name))
}

func TestStripTriggerEmptyNameIsNoop(t *testing.T) {
	input := "  hello world  "
	require.Equal(t, "hello world", StripTrigger(input, ""))
}
