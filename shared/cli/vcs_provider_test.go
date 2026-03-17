package cli

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseVCSProviderDefault(t *testing.T) {
	provider, host, err := ParseVCSProvider("")
	require.Error(t, err)
	require.Empty(t, provider)
	require.Empty(t, host)
}

func TestParseVCSProviderGitLabNoHost(t *testing.T) {
	provider, host, err := ParseVCSProvider("gitlab")
	require.NoError(t, err)
	require.Equal(t, "gitlab", provider)
	require.Empty(t, host)
}

func TestParseVCSProviderGitHub(t *testing.T) {
	provider, host, err := ParseVCSProvider("github")
	require.NoError(t, err)
	require.Equal(t, "github", provider)
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

func TestParseVCSProviderUnsupported(t *testing.T) {
	_, _, err := ParseVCSProvider("bitbucket")
	require.Error(t, err)
	require.Contains(t, err.Error(), "supported")
}

func TestResolveVCSProviderFromRepoGitHubURL(t *testing.T) {
	provider, host, err := ResolveVCSProviderFromRepo("https://github.com/owner/repo.git")
	require.NoError(t, err)
	require.Equal(t, "github", provider)
	require.Empty(t, host)
}

func TestResolveVCSProviderFromRepoGitHubSSH(t *testing.T) {
	provider, host, err := ResolveVCSProviderFromRepo("git@github.com:owner/repo.git")
	require.NoError(t, err)
	require.Equal(t, "github", provider)
	require.Empty(t, host)
}

func TestResolveVCSProviderFromRepoGitLabURL(t *testing.T) {
	provider, host, err := ResolveVCSProviderFromRepo("https://gitlab.example.com/group/project.git")
	require.NoError(t, err)
	require.Equal(t, "gitlab", provider)
	require.Equal(t, "gitlab.example.com", host)
}

func TestResolveVCSProviderFromRepoGitLabSSH(t *testing.T) {
	provider, host, err := ResolveVCSProviderFromRepo("git@gitlab.example.com:group/project.git")
	require.NoError(t, err)
	require.Equal(t, "gitlab", provider)
	require.Equal(t, "gitlab.example.com", host)
}

func TestResolveVCSProviderFromRepoAmbiguous(t *testing.T) {
	_, _, err := ResolveVCSProviderFromRepo("owner/repo")
	require.Error(t, err)
}

func TestResolveVCSProviderFromRepoUnsupportedHost(t *testing.T) {
	_, _, err := ResolveVCSProviderFromRepo("https://bitbucket.org/owner/repo.git")
	require.Error(t, err)
}
