package skillupdate

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
)

func TestUpdateWithDirectPath(t *testing.T) {
	tempDir := t.TempDir()
	targetDir := filepath.Join(tempDir, "peer")
	require.NoError(t, os.MkdirAll(targetDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(targetDir, "old.txt"), []byte("old"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(targetDir, "SKILL.md"), []byte("old-skill"), 0o644))

	source := fstest.MapFS{
		"peer/SKILL.md":          &fstest.MapFile{Data: []byte("new-skill")},
		"peer/references/a.md":   &fstest.MapFile{Data: []byte("ref")},
		"peer/references/b.txt":  &fstest.MapFile{Data: []byte("ref2")},
		"peer/subdir/info.json":  &fstest.MapFile{Data: []byte("{}")},
		"peer/subdir/nested.txt": &fstest.MapFile{Data: []byte("nested")},
	}

	updater := NewUpdater(&UpdateDeps{
		SourceFS:   source,
		SourceRoot: "peer",
	})

	results, err := updater.Update(context.Background(), []string{targetDir})
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.NoError(t, results[0].Err)

	_, err = os.Stat(filepath.Join(targetDir, "old.txt"))
	require.Error(t, err)

	content, err := os.ReadFile(filepath.Join(targetDir, "SKILL.md"))
	require.NoError(t, err)
	require.Equal(t, "new-skill", string(content))
}

func TestUpdateWithRootPath(t *testing.T) {
	tempDir := t.TempDir()
	rootDir := filepath.Join(tempDir, "skills-root")
	targetDir := filepath.Join(rootDir, "peer")
	require.NoError(t, os.MkdirAll(targetDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(targetDir, "SKILL.md"), []byte("old"), 0o644))

	source := fstest.MapFS{
		"peer/SKILL.md": &fstest.MapFile{Data: []byte("new")},
	}

	updater := NewUpdater(&UpdateDeps{
		SourceFS:   source,
		SourceRoot: "peer",
	})

	results, err := updater.Update(context.Background(), []string{rootDir})
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, targetDir, results[0].Path)
	require.NoError(t, results[0].Err)
}

func TestUpdateAutoDiscovery(t *testing.T) {
	tempDir := t.TempDir()
	projectRoot := filepath.Join(tempDir, "repo")
	subdir := filepath.Join(projectRoot, "subdir")
	require.NoError(t, os.MkdirAll(subdir, 0o755))

	homeDir := filepath.Join(tempDir, "home")
	require.NoError(t, os.MkdirAll(homeDir, 0o755))

	xdgHome := filepath.Join(tempDir, "xdg")
	require.NoError(t, os.MkdirAll(xdgHome, 0o755))

	projectTarget := filepath.Join(projectRoot, ".agents", "skills", "peer")
	userTarget := filepath.Join(homeDir, ".claude", "skills", "peer")
	xdgTarget := filepath.Join(xdgHome, ".codex", "skills", "peer")

	targets := []string{projectTarget, userTarget, xdgTarget}
	for _, target := range targets {
		require.NoError(t, os.MkdirAll(target, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(target, "SKILL.md"), []byte("old"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(target, "old.txt"), []byte("old"), 0o644))
	}

	source := fstest.MapFS{
		"peer/SKILL.md": &fstest.MapFile{Data: []byte("new")},
		"peer/new.txt":  &fstest.MapFile{Data: []byte("data")},
	}

	updater := NewUpdater(&UpdateDeps{
		HomeDir:        homeDir,
		Cwd:            subdir,
		XDGConfigHome:  xdgHome,
		ResolveGitRoot: func(string) (string, error) { return projectRoot, nil },
		SourceFS:       source,
		SourceRoot:     "peer",
	})

	results, err := updater.Update(context.Background(), nil)
	require.NoError(t, err)
	require.Len(t, results, 3)

	for _, target := range targets {
		content, err := os.ReadFile(filepath.Join(target, "SKILL.md"))
		require.NoError(t, err)
		require.Equal(t, "new", string(content))

		_, err = os.Stat(filepath.Join(target, "old.txt"))
		require.Error(t, err)

		_, err = os.Stat(filepath.Join(target, "new.txt"))
		require.NoError(t, err)
	}
}

func TestResolveTargetsFromPathsErrors(t *testing.T) {
	tempDir := t.TempDir()
	updater := NewUpdater(&UpdateDeps{
		HomeDir:    tempDir,
		SourceFS:   fstest.MapFS{"peer/SKILL.md": &fstest.MapFile{Data: []byte("x")}},
		SourceRoot: "peer",
	})

	_, err := updater.Update(context.Background(), []string{"~/missing"})
	require.Error(t, err)
}

func TestExpandHomeMissingHome(t *testing.T) {
	_, err := expandHome("~/path", "")
	require.Error(t, err)
}

func TestUpdateWithContextCanceled(t *testing.T) {
	tempDir := t.TempDir()
	targetDir := filepath.Join(tempDir, "peer")
	require.NoError(t, os.MkdirAll(targetDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(targetDir, "SKILL.md"), []byte("old"), 0o644))

	source := fstest.MapFS{
		"peer/SKILL.md": &fstest.MapFile{Data: []byte("new")},
	}

	updater := NewUpdater(&UpdateDeps{
		SourceFS:   source,
		SourceRoot: "peer",
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := updater.Update(ctx, []string{targetDir})
	require.Error(t, err)
}

func TestUpdateSkipInvalidGitRoot(t *testing.T) {
	tempDir := t.TempDir()
	updater := NewUpdater(&UpdateDeps{
		HomeDir:        tempDir,
		Cwd:            tempDir,
		ResolveGitRoot: func(string) (string, error) { return "", errors.New("no git") },
		SourceFS:       fstest.MapFS{"peer/SKILL.md": &fstest.MapFile{Data: []byte("x")}},
		SourceRoot:     "peer",
	})

	results, err := updater.Update(context.Background(), nil)
	require.NoError(t, err)
	require.Empty(t, results)
}

func TestCopyFSUsesSourceMode(t *testing.T) {
	tempDir := t.TempDir()
	targetDir := filepath.Join(tempDir, "peer")
	require.NoError(t, os.MkdirAll(targetDir, 0o755))

	source := fstest.MapFS{
		"peer/SKILL.md": &fstest.MapFile{Data: []byte("new"), Mode: fs.FileMode(0o600)},
	}

	err := copyFS(source, "peer", targetDir)
	require.NoError(t, err)

	info, err := os.Stat(filepath.Join(targetDir, "SKILL.md"))
	require.NoError(t, err)
	require.Equal(t, fs.FileMode(0o600), info.Mode().Perm())
}
