package text

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestContainsTriggerMatchesAtOrSlash(t *testing.T) {
	name := "PeerBot"

	require.True(t, ContainsTrigger("please /peerbot do", name))
	require.True(t, ContainsTrigger("hi @PEERBOT ok", name))
	require.True(t, ContainsTrigger("(@peerbot) thanks", name))
	require.False(t, ContainsTrigger("just peerbot", name))
	require.False(t, ContainsTrigger("/peerbotty", name))
	require.False(t, ContainsTrigger("@peerbotty", name))
}

func TestStripTriggerRemovesAllOccurrences(t *testing.T) {
	name := "peerbot"
	input := "  @peerbot  please\n /peerbot  do  \n"

	require.Equal(t, "please\ndo", StripTrigger(input, name))
}

func TestStripTriggerEmptyNameIsNoop(t *testing.T) {
	input := "  hello world  "
	require.Equal(t, "hello world", StripTrigger(input, ""))
}
