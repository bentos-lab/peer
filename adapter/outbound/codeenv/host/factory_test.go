package host

import (
	"context"
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

func TestFactoryNewLocalWorkspaceUsesCurrentDirectory(t *testing.T) {
	runner := commandrunner.NewDummyCommandRunner()
	factory := NewFactory(FactoryConfig{
		Runner: runner,
		Getwd: func() (string, error) {
			return "/workspace/current", nil
		},
	})

	environment, err := factory.New(context.Background(), domain.CodeEnvironmentInitOptions{})
	require.NoError(t, err)
	require.IsType(t, &HostCodeEnvironment{}, environment)
	require.NoError(t, runner.VerifyDone())
}
