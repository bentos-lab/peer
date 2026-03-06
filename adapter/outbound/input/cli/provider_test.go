package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"bentos-backend/usecase"
	"github.com/stretchr/testify/require"
)

type fakeChangeDetector struct {
	staged    []string
	unstaged  []string
	untracked []string
	err       error
}

func (f *fakeChangeDetector) ListStaged(_ context.Context) ([]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.staged, nil
}

func (f *fakeChangeDetector) ListUnstaged(_ context.Context) ([]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.unstaged, nil
}

func (f *fakeChangeDetector) ListUntracked(_ context.Context) ([]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.untracked, nil
}

func TestProvider_LoadChangeSnapshotManualOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.go")
	require.NoError(t, os.WriteFile(path, []byte("package a"), 0o644))

	provider := NewProvider(&fakeChangeDetector{})
	result, err := provider.LoadChangeSnapshot(context.Background(), usecase.ChangeRequestRequest{
		Repository: "org/repo",
		Metadata: map[string]string{
			MetadataKeyChangedFiles: path,
		},
	})
	require.NoError(t, err)
	require.Len(t, result.ChangedFiles, 1)
	require.Equal(t, path, result.ChangedFiles[0].Path)
}

func TestProvider_LoadChangeSnapshotAutoDefaultStagedOnly(t *testing.T) {
	dir := t.TempDir()
	staged := filepath.Join(dir, "staged.go")
	require.NoError(t, os.WriteFile(staged, []byte("package staged"), 0o644))

	provider := NewProvider(&fakeChangeDetector{
		staged: []string{staged},
	})
	result, err := provider.LoadChangeSnapshot(context.Background(), usecase.ChangeRequestRequest{
		Metadata: map[string]string{},
	})
	require.NoError(t, err)
	require.Len(t, result.ChangedFiles, 1)
	require.Equal(t, staged, result.ChangedFiles[0].Path)
}

func TestProvider_LoadChangeSnapshotAutoAllIncludesUnstaged(t *testing.T) {
	dir := t.TempDir()
	staged := filepath.Join(dir, "staged.go")
	unstaged := filepath.Join(dir, "unstaged.go")
	require.NoError(t, os.WriteFile(staged, []byte("package staged"), 0o644))
	require.NoError(t, os.WriteFile(unstaged, []byte("package unstaged"), 0o644))

	provider := NewProvider(&fakeChangeDetector{
		staged:   []string{staged},
		unstaged: []string{unstaged},
	})
	result, err := provider.LoadChangeSnapshot(context.Background(), usecase.ChangeRequestRequest{
		Metadata: map[string]string{
			MetadataKeyAutoIncludeAll: "true",
		},
	})
	require.NoError(t, err)
	require.Len(t, result.ChangedFiles, 2)
}

func TestProvider_LoadChangeSnapshotAutoIncludesUntracked(t *testing.T) {
	dir := t.TempDir()
	staged := filepath.Join(dir, "staged.go")
	untracked := filepath.Join(dir, "untracked.go")
	require.NoError(t, os.WriteFile(staged, []byte("package staged"), 0o644))
	require.NoError(t, os.WriteFile(untracked, []byte("package untracked"), 0o644))

	provider := NewProvider(&fakeChangeDetector{
		staged:    []string{staged},
		untracked: []string{untracked},
	})
	result, err := provider.LoadChangeSnapshot(context.Background(), usecase.ChangeRequestRequest{
		Metadata: map[string]string{
			MetadataKeyAutoIncludeUntracked: "true",
		},
	})
	require.NoError(t, err)
	require.Len(t, result.ChangedFiles, 2)
}

func TestProvider_LoadChangeSnapshotAutoAllAndUntrackedDedupes(t *testing.T) {
	dir := t.TempDir()
	staged := filepath.Join(dir, "staged.go")
	unstaged := filepath.Join(dir, "unstaged.go")
	untracked := filepath.Join(dir, "untracked.go")
	require.NoError(t, os.WriteFile(staged, []byte("package staged"), 0o644))
	require.NoError(t, os.WriteFile(unstaged, []byte("package unstaged"), 0o644))
	require.NoError(t, os.WriteFile(untracked, []byte("package untracked"), 0o644))

	provider := NewProvider(&fakeChangeDetector{
		staged:    []string{staged, unstaged},
		unstaged:  []string{unstaged},
		untracked: []string{untracked},
	})
	result, err := provider.LoadChangeSnapshot(context.Background(), usecase.ChangeRequestRequest{
		Metadata: map[string]string{
			MetadataKeyAutoIncludeAll:       "true",
			MetadataKeyAutoIncludeUntracked: "true",
		},
	})
	require.NoError(t, err)
	require.Len(t, result.ChangedFiles, 3)
}

func TestProvider_LoadChangeSnapshotAutoSkipsDeletedFiles(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "exists.go")
	deleted := filepath.Join(dir, "deleted.go")
	require.NoError(t, os.WriteFile(existing, []byte("package exists"), 0o644))

	provider := NewProvider(&fakeChangeDetector{
		staged: []string{existing, deleted},
	})
	result, err := provider.LoadChangeSnapshot(context.Background(), usecase.ChangeRequestRequest{})
	require.NoError(t, err)
	require.Len(t, result.ChangedFiles, 1)
	require.Equal(t, existing, result.ChangedFiles[0].Path)
}

func TestProvider_LoadChangeSnapshotAutoEmptyReturnsError(t *testing.T) {
	provider := NewProvider(&fakeChangeDetector{})
	_, err := provider.LoadChangeSnapshot(context.Background(), usecase.ChangeRequestRequest{})
	require.Error(t, err)
	require.ErrorIs(t, err, errNoChangedFiles)
}

func TestProvider_LoadChangeSnapshotManualMissingFileReturnsError(t *testing.T) {
	provider := NewProvider(&fakeChangeDetector{})
	_, err := provider.LoadChangeSnapshot(context.Background(), usecase.ChangeRequestRequest{
		Metadata: map[string]string{
			MetadataKeyChangedFiles: "missing.go",
		},
	})
	require.Error(t, err)
}

func TestProvider_LoadChangeSnapshotReturnsDetectorError(t *testing.T) {
	provider := NewProvider(&fakeChangeDetector{err: errors.New("detector failed")})
	_, err := provider.LoadChangeSnapshot(context.Background(), usecase.ChangeRequestRequest{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "detector failed")
}
