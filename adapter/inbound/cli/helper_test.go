package cli

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveBoolPrimary(t *testing.T) {
	primary := true
	fallback := false

	result := ResolveBool(&primary, &fallback, false)

	require.True(t, result)
}

func TestResolveBoolFallback(t *testing.T) {
	fallback := true

	result := ResolveBool(nil, &fallback, false)

	require.True(t, result)
}

func TestResolveBoolDefault(t *testing.T) {
	result := ResolveBool(nil, nil, true)

	require.True(t, result)
}

func TestNormalizeRepoGitLabUsesOverrideHost(t *testing.T) {
	repo, repoURL, _, err := normalizeRepo("gitlab", "gitlab.example.com", "group/project")
	require.NoError(t, err)
	require.Equal(t, "group/project", repo)
	require.Equal(t, "https://gitlab.example.com/group/project.git", repoURL)
}

func TestNormalizeRepoGitLabUsesDefaultHostWhenOverrideEmpty(t *testing.T) {
	t.Setenv("GITLAB_HOST", "gitlab.internal")
	repo, repoURL, _, err := normalizeRepo("gitlab", "", "group/project")
	require.NoError(t, err)
	require.Equal(t, "group/project", repo)
	require.Equal(t, "https://gitlab.internal/group/project.git", repoURL)
}
