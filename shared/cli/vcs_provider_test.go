package cli

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseVCSProviderDefault(t *testing.T) {
	provider, host, err := ParseVCSProvider("")
	require.NoError(t, err)
	require.Equal(t, "github", provider)
	require.Empty(t, host)
}

func TestParseVCSProviderGitLabNoHost(t *testing.T) {
	provider, host, err := ParseVCSProvider("gitlab")
	require.NoError(t, err)
	require.Equal(t, "gitlab", provider)
	require.Empty(t, host)
}

func TestParseVCSProviderGitLabWithHost(t *testing.T) {
	provider, host, err := ParseVCSProvider("gitlab:gitlab.example.com")
	require.NoError(t, err)
	require.Equal(t, "gitlab", provider)
	require.Equal(t, "gitlab.example.com", host)
}

func TestParseVCSProviderGitLabEmptyHost(t *testing.T) {
	_, _, err := ParseVCSProvider("gitlab:")
	require.Error(t, err)
}
