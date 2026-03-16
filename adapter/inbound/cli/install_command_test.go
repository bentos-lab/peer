package cli

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type installCallRecorder struct {
	calls []string
}

func (r *installCallRecorder) EnsureGhInstalled(context.Context) error {
	r.calls = append(r.calls, "gh-install")
	return nil
}

func (r *installCallRecorder) EnsureGhAuthenticated(context.Context) error {
	r.calls = append(r.calls, "gh-auth")
	return nil
}

func (r *installCallRecorder) EnsureOpencodeInstalled(context.Context) error {
	r.calls = append(r.calls, "opencode-install")
	return nil
}

func (r *installCallRecorder) EnsureGlabInstalled(context.Context) error {
	r.calls = append(r.calls, "glab-install")
	return nil
}

func (r *installCallRecorder) EnsureGlabAuthenticated(context.Context) error {
	r.calls = append(r.calls, "glab-auth")
	return nil
}

func TestInstallCommandGhLogin(t *testing.T) {
	recorder := &installCallRecorder{}
	cmd := &InstallCommand{installer: recorder}

	err := cmd.InstallGh(context.Background(), true)
	require.NoError(t, err)
	require.Equal(t, []string{"gh-install", "gh-auth"}, recorder.calls)
}

func TestInstallCommandOpencode(t *testing.T) {
	recorder := &installCallRecorder{}
	cmd := &InstallCommand{installer: recorder}

	err := cmd.InstallOpencode(context.Background())
	require.NoError(t, err)
	require.Equal(t, []string{"opencode-install"}, recorder.calls)
}

func TestInstallCommandGlabLogin(t *testing.T) {
	recorder := &installCallRecorder{}
	cmd := &InstallCommand{installer: recorder}

	err := cmd.InstallGlab(context.Background(), true)
	require.NoError(t, err)
	require.Equal(t, []string{"glab-install", "glab-auth"}, recorder.calls)
}

func TestInstallCommandQuickstart(t *testing.T) {
	recorder := &installCallRecorder{}
	cmd := &InstallCommand{installer: recorder}

	err := cmd.InstallQuickstart(context.Background())
	require.NoError(t, err)
	require.Equal(t, []string{"gh-install", "gh-auth", "opencode-install"}, recorder.calls)
}
