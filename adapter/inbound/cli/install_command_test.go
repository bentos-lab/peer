package cli

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type installCallRecorder struct {
	calls             []string
	ghInstalled       bool
	glabInstalled     bool
	opencodeInstalled bool
	gitInstalled      bool
	ghAuthenticated   bool
	glabAuthenticated bool
}

func (r *installCallRecorder) IsGhInstalled() bool {
	return r.ghInstalled
}

func (r *installCallRecorder) IsGhAuthenticated(context.Context) (bool, error) {
	return r.ghAuthenticated, nil
}

func (r *installCallRecorder) EnsureGhInstalled(context.Context) error {
	r.calls = append(r.calls, "gh-install")
	return nil
}

func (r *installCallRecorder) EnsureGhAuthenticated(context.Context) error {
	r.calls = append(r.calls, "gh-auth")
	return nil
}

func (r *installCallRecorder) IsOpencodeInstalled() bool {
	return r.opencodeInstalled
}

func (r *installCallRecorder) EnsureOpencodeInstalled(context.Context) error {
	r.calls = append(r.calls, "opencode-install")
	return nil
}

func (r *installCallRecorder) IsGlabInstalled() bool {
	return r.glabInstalled
}

func (r *installCallRecorder) IsGlabAuthenticated(context.Context) (bool, error) {
	return r.glabAuthenticated, nil
}

func (r *installCallRecorder) EnsureGlabInstalled(context.Context) error {
	r.calls = append(r.calls, "glab-install")
	return nil
}

func (r *installCallRecorder) EnsureGlabAuthenticated(context.Context) error {
	r.calls = append(r.calls, "glab-auth")
	return nil
}

func (r *installCallRecorder) IsGitInstalled() bool {
	return r.gitInstalled
}

func (r *installCallRecorder) EnsureGitInstalled(context.Context) error {
	r.calls = append(r.calls, "git-install")
	return nil
}

func TestInstallCommandGhLogin(t *testing.T) {
	recorder := &installCallRecorder{}
	cmd := &InstallCommand{gh: recorder}

	outcome, err := cmd.InstallGh(context.Background(), true)
	require.NoError(t, err)
	require.Equal(t, []string{"gh-install", "gh-auth"}, recorder.calls)
	require.True(t, outcome.Installed)
	require.False(t, outcome.AlreadyAuthenticated)
}

func TestInstallCommandOpencode(t *testing.T) {
	recorder := &installCallRecorder{}
	cmd := &InstallCommand{opencode: recorder}

	outcome, err := cmd.InstallOpencode(context.Background())
	require.NoError(t, err)
	require.Equal(t, []string{"opencode-install"}, recorder.calls)
	require.True(t, outcome.Installed)
}

func TestInstallCommandGlabLogin(t *testing.T) {
	recorder := &installCallRecorder{}
	cmd := &InstallCommand{glab: recorder}

	outcome, err := cmd.InstallGlab(context.Background(), true)
	require.NoError(t, err)
	require.Equal(t, []string{"glab-install", "glab-auth"}, recorder.calls)
	require.True(t, outcome.Installed)
	require.False(t, outcome.AlreadyAuthenticated)
}

func TestInstallCommandGit(t *testing.T) {
	recorder := &installCallRecorder{}
	cmd := &InstallCommand{git: recorder}

	outcome, err := cmd.InstallGit(context.Background())
	require.NoError(t, err)
	require.Equal(t, []string{"git-install"}, recorder.calls)
	require.True(t, outcome.Installed)
}

func TestInstallCommandQuickstart(t *testing.T) {
	recorder := &installCallRecorder{}
	cmd := &InstallCommand{gh: recorder, opencode: recorder}

	outcome, err := cmd.InstallQuickstart(context.Background())
	require.NoError(t, err)
	require.Equal(t, []string{"gh-install", "gh-auth", "opencode-install"}, recorder.calls)
	require.True(t, outcome.Gh.Installed)
	require.True(t, outcome.Opencode.Installed)
}

func TestInstallCommandSkipsInstallWhenAlreadyPresent(t *testing.T) {
	recorder := &installCallRecorder{ghInstalled: true}
	cmd := &InstallCommand{gh: recorder}

	outcome, err := cmd.InstallGh(context.Background(), false)
	require.NoError(t, err)
	require.Empty(t, recorder.calls)
	require.False(t, outcome.Installed)
	require.False(t, outcome.AlreadyAuthenticated)
}

func TestInstallCommandSkipsAuthWhenAlreadyAuthenticated(t *testing.T) {
	recorder := &installCallRecorder{ghAuthenticated: true}
	cmd := &InstallCommand{gh: recorder}

	outcome, err := cmd.InstallGh(context.Background(), true)
	require.NoError(t, err)
	require.Equal(t, []string{"gh-install"}, recorder.calls)
	require.True(t, outcome.Installed)
	require.True(t, outcome.AlreadyAuthenticated)
}
