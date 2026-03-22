package host

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/bentos-lab/peer/adapter/outbound/commandrunner"
	"github.com/bentos-lab/peer/domain"

	"github.com/stretchr/testify/require"
)

func TestFactoryNewRemoteWorkspacePreparesClone(t *testing.T) {
	runner := commandrunner.NewDummyCommandRunner()
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"clone", "--depth", "1", "https://github.com/org/repo.git", "/home/test/.bentos-labtmp/workspace-1"},
		},
		Result: commandrunner.Result{Stdout: []byte("cloned")},
	})

	factory := NewFactory(FactoryConfig{
		Runner: runner,
		MakeTempDir: func() (string, error) {
			return "/home/test/.bentos-labtmp/workspace-1", nil
		},
	})

	environment, err := factory.New(context.Background(), domain.CodeEnvironmentInitOptions{
		RepoURL: "https://github.com/org/repo.git",
	})
	require.NoError(t, err)
	require.IsType(t, &HostCodeEnvironment{}, environment)
	require.NoError(t, runner.VerifyDone())
}

func TestFactoryNewLocalWorkspaceUsesTempCopy(t *testing.T) {
	runner := commandrunner.NewDummyCommandRunner()
	sourceDir := t.TempDir()
	tempDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "README.md"), []byte("hello"), 0o644))
	factory := NewFactory(FactoryConfig{
		Runner: runner,
		Getwd: func() (string, error) {
			return sourceDir, nil
		},
		MakeTempDir: func() (string, error) {
			return tempDir, nil
		},
	})

	environment, err := factory.New(context.Background(), domain.CodeEnvironmentInitOptions{})
	require.NoError(t, err)
	hostEnv, ok := environment.(*HostCodeEnvironment)
	require.True(t, ok)
	require.Equal(t, tempDir, hostEnv.workspaceDir)
	_, statErr := os.Stat(filepath.Join(tempDir, "README.md"))
	require.NoError(t, statErr)
	require.NoError(t, runner.VerifyDone())
}

func TestFactoryNewLocalWorkspaceUsesCwdWhenFlagSet(t *testing.T) {
	runner := commandrunner.NewDummyCommandRunner()
	sourceDir := t.TempDir()
	makeTempDirCalled := false

	factory := NewFactory(FactoryConfig{
		Runner: runner,
		Getwd: func() (string, error) {
			return sourceDir, nil
		},
		MakeTempDir: func() (string, error) {
			makeTempDirCalled = true
			return t.TempDir(), nil
		},
	})

	environment, err := factory.New(context.Background(), domain.CodeEnvironmentInitOptions{
		UseCwd: true,
	})
	require.NoError(t, err)
	hostEnv, ok := environment.(*HostCodeEnvironment)
	require.True(t, ok)
	require.Equal(t, sourceDir, hostEnv.workspaceDir)
	require.False(t, hostEnv.cleanup)
	require.False(t, makeTempDirCalled)
	require.NoError(t, runner.VerifyDone())
}
