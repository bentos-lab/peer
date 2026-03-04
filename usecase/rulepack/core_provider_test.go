package rulepack

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCoreRulePackProvider_LoadsAndRendersEmbeddedMarkdownTemplate(t *testing.T) {
	provider := NewCoreRulePackProvider()

	pack, err := provider.CorePack(context.Background())
	require.NoError(t, err)
	require.Equal(t, "core", pack.ID)
	require.Equal(t, "v1", pack.Version)
	require.Len(t, pack.Instructions, 1)
	require.Contains(t, pack.Instructions[0], "Review language: English")
	require.Contains(t, pack.Instructions[0], "Potential bugs or correctness risks")
}
