package host

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bentos-lab/peer/adapter/outbound/commandrunner"
	"github.com/bentos-lab/peer/domain"

	"github.com/stretchr/testify/require"
)

func TestHostCodeEnvironment_SetupAgentUnsupportedAgent(t *testing.T) {
	env := NewHostCodeEnvironment(HostCodeEnvironmentConfig{
		Runner: commandrunner.NewDummyCommandRunner(),
		Getwd: func() (string, error) {
			return "/workspace/current", nil
		},
	})

	_, err := env.SetupAgent(context.Background(), domain.CodingAgentSetupOptions{
		Agent: "codex",
	})
	require.EqualError(t, err, "unsupported coding agent: codex")
}

func TestHostCodeEnvironment_SetupAgentLocalWorkspaceNoRef(t *testing.T) {
	runner := commandrunner.NewDummyCommandRunner()
	makeTempDirCalled := false
	env := NewHostCodeEnvironment(HostCodeEnvironmentConfig{
		Runner: runner,
		Getwd: func() (string, error) {
			return "/workspace/current", nil
		},
		MakeTempDir: func() (string, error) {
			makeTempDirCalled = true
			return "/home/test/.bentos-labtmp/workspace-1", nil
		},
	})

	agent, err := env.SetupAgent(context.Background(), domain.CodingAgentSetupOptions{
		Agent: "opencode",
	})
	require.NoError(t, err)

	hostAgent, ok := agent.(*HostOpencodeAgent)
	require.True(t, ok)
	require.Equal(t, "/workspace/current", hostAgent.workspaceDir)
	require.False(t, makeTempDirCalled)
	require.NoError(t, runner.VerifyDone())
}

func TestHostCodeEnvironment_SetupAgentLocalWorkspaceStagedTokenRefSkipsSync(t *testing.T) {
	runner := commandrunner.NewDummyCommandRunner()
	env := NewHostCodeEnvironment(HostCodeEnvironmentConfig{
		Runner: runner,
		Getwd: func() (string, error) {
			return "/workspace/current", nil
		},
	})

	agent, err := env.SetupAgent(context.Background(), domain.CodingAgentSetupOptions{
		Agent: "opencode",
		Ref:   "@staged",
	})
	require.NoError(t, err)

	hostAgent, ok := agent.(*HostOpencodeAgent)
	require.True(t, ok)
	require.Equal(t, "/workspace/current", hostAgent.workspaceDir)
	require.NoError(t, runner.VerifyDone())
}

func TestHostCodeEnvironment_SetupAgentLocalWorkspaceAllTokenRefSkipsSync(t *testing.T) {
	runner := commandrunner.NewDummyCommandRunner()
	env := NewHostCodeEnvironment(HostCodeEnvironmentConfig{
		Runner: runner,
		Getwd: func() (string, error) {
			return "/workspace/current", nil
		},
	})

	agent, err := env.SetupAgent(context.Background(), domain.CodingAgentSetupOptions{
		Agent: "opencode",
		Ref:   "@all",
	})
	require.NoError(t, err)

	hostAgent, ok := agent.(*HostOpencodeAgent)
	require.True(t, ok)
	require.Equal(t, "/workspace/current", hostAgent.workspaceDir)
	require.NoError(t, runner.VerifyDone())
}

func TestHostCodeEnvironment_SetupAgentRemoteWorkspaceNoRef(t *testing.T) {
	runner := commandrunner.NewDummyCommandRunner()

	env := NewHostCodeEnvironment(HostCodeEnvironmentConfig{
		Runner:       runner,
		WorkspaceDir: "/home/test/.bentos-labtmp/workspace-1",
		IsRemote:     true,
	})

	agent, err := env.SetupAgent(context.Background(), domain.CodingAgentSetupOptions{
		Agent: "OpenCode",
	})
	require.NoError(t, err)

	hostAgent, ok := agent.(*HostOpencodeAgent)
	require.True(t, ok)
	require.Equal(t, "/home/test/.bentos-labtmp/workspace-1", hostAgent.workspaceDir)
	require.NoError(t, runner.VerifyDone())
}

func TestHostCodeEnvironment_SetupAgentLocalWorkspaceRef(t *testing.T) {
	runner := commandrunner.NewDummyCommandRunner()
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", "/workspace/current", "config", "--get", "remote.origin.url"},
		},
		Result: commandrunner.Result{Stdout: []byte("https://github.com/example/repo.git")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"clone", "--depth", "1", "https://github.com/example/repo.git", "/home/test/.bentos-labtmp/workspace-1"},
		},
		Result: commandrunner.Result{Stdout: []byte("cloned")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", "/home/test/.bentos-labtmp/workspace-1", "rev-parse", "HEAD"},
		},
		Result: commandrunner.Result{Stdout: []byte("head123")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", "/home/test/.bentos-labtmp/workspace-1", "rev-parse", "--verify", "feature/ref^{commit}"},
		},
		Result: commandrunner.Result{Stdout: []byte("abc123")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", "/home/test/.bentos-labtmp/workspace-1", "checkout", "feature/ref"},
		},
		Result: commandrunner.Result{Stdout: []byte("checked out")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", "/home/test/.bentos-labtmp/workspace-1", "rev-parse", "HEAD"},
		},
		Result: commandrunner.Result{Stdout: []byte("head456")},
	})

	env := NewHostCodeEnvironment(HostCodeEnvironmentConfig{
		Runner: runner,
		Getwd: func() (string, error) {
			return "/workspace/current", nil
		},
		MakeTempDir: func() (string, error) {
			return "/home/test/.bentos-labtmp/workspace-1", nil
		},
	})

	agent, err := env.SetupAgent(context.Background(), domain.CodingAgentSetupOptions{
		Agent: "opencode",
		Ref:   "feature/ref",
	})
	require.NoError(t, err)

	hostAgent, ok := agent.(*HostOpencodeAgent)
	require.True(t, ok)
	require.Equal(t, "/home/test/.bentos-labtmp/workspace-1", hostAgent.workspaceDir)
	require.NoError(t, runner.VerifyDone())
}

func TestHostCodeEnvironment_ReadFileWorkspace(t *testing.T) {
	workspaceDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workspaceDir, "README.md"), []byte("hello"), 0o644))

	runner := commandrunner.NewDummyCommandRunner()
	env := NewHostCodeEnvironment(HostCodeEnvironmentConfig{
		Runner: runner,
		Getwd: func() (string, error) {
			return workspaceDir, nil
		},
	})

	content, found, err := env.ReadFile(context.Background(), "README.md", "@staged")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "hello", content)
	require.NoError(t, runner.VerifyDone())
}

func TestHostCodeEnvironment_ReadFileWorkspaceIndexFallback(t *testing.T) {
	workspaceDir := t.TempDir()

	runner := commandrunner.NewDummyCommandRunner()
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", workspaceDir, "show", ":README.md"},
		},
		Result: commandrunner.Result{
			Stdout: []byte("from-index"),
		},
	})

	env := NewHostCodeEnvironment(HostCodeEnvironmentConfig{
		Runner: runner,
		Getwd: func() (string, error) {
			return workspaceDir, nil
		},
	})

	content, found, err := env.ReadFile(context.Background(), "README.md", "@staged")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "from-index", content)
	require.NoError(t, runner.VerifyDone())
}

func TestHostCodeEnvironment_ReadFileWorkspaceMissingUsesAmbiguousArgument(t *testing.T) {
	workspaceDir := t.TempDir()

	runner := commandrunner.NewDummyCommandRunner()
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", workspaceDir, "show", ":.peer/rules.md"},
		},
		Result: commandrunner.Result{
			Stderr: []byte("fatal: ambiguous argument ':.peer/rules.md': unknown revision or path not in the working tree."),
		},
		Err: errors.New("exit status 128"),
	})

	env := NewHostCodeEnvironment(HostCodeEnvironmentConfig{
		Runner: runner,
		Getwd: func() (string, error) {
			return workspaceDir, nil
		},
	})

	content, found, err := env.ReadFile(context.Background(), ".peer/rules.md", "@staged")
	require.NoError(t, err)
	require.False(t, found)
	require.Equal(t, "", content)
	require.NoError(t, runner.VerifyDone())
}

func TestHostCodeEnvironment_ReadFileRef(t *testing.T) {
	workspaceDir := t.TempDir()

	runner := commandrunner.NewDummyCommandRunner()
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", workspaceDir, "config", "--get", "remote.origin.url"},
		},
		Result: commandrunner.Result{Stdout: []byte("https://github.com/example/repo.git")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"clone", "--depth", "1", "https://github.com/example/repo.git", "/home/test/.bentos-labtmp/workspace-1"},
		},
		Result: commandrunner.Result{Stdout: []byte("cloned")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", "/home/test/.bentos-labtmp/workspace-1", "cat-file", "-e", "HEAD:README.md"},
		},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", "/home/test/.bentos-labtmp/workspace-1", "show", "HEAD:README.md"},
		},
		Result: commandrunner.Result{
			Stdout: []byte("from-ref"),
		},
	})

	env := NewHostCodeEnvironment(HostCodeEnvironmentConfig{
		Runner: runner,
		Getwd: func() (string, error) {
			return workspaceDir, nil
		},
		MakeTempDir: func() (string, error) {
			return "/home/test/.bentos-labtmp/workspace-1", nil
		},
	})

	content, found, err := env.ReadFile(context.Background(), "README.md", "HEAD")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "from-ref", content)
	require.NoError(t, runner.VerifyDone())
}

func TestHostCodeEnvironment_ReadFileRefMissing(t *testing.T) {
	workspaceDir := t.TempDir()

	runner := commandrunner.NewDummyCommandRunner()
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", workspaceDir, "config", "--get", "remote.origin.url"},
		},
		Result: commandrunner.Result{Stdout: []byte("https://github.com/example/repo.git")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"clone", "--depth", "1", "https://github.com/example/repo.git", "/home/test/.bentos-labtmp/workspace-1"},
		},
		Result: commandrunner.Result{Stdout: []byte("cloned")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", "/home/test/.bentos-labtmp/workspace-1", "cat-file", "-e", "HEAD:missing.txt"},
		},
		Result: commandrunner.Result{
			Stderr: []byte("fatal: Path 'missing.txt' does not exist in 'HEAD'"),
		},
		Err: errors.New("exit status 1"),
	})

	env := NewHostCodeEnvironment(HostCodeEnvironmentConfig{
		Runner: runner,
		Getwd: func() (string, error) {
			return workspaceDir, nil
		},
		MakeTempDir: func() (string, error) {
			return "/home/test/.bentos-labtmp/workspace-1", nil
		},
	})

	content, found, err := env.ReadFile(context.Background(), "missing.txt", "HEAD")
	require.NoError(t, err)
	require.False(t, found)
	require.Equal(t, "", content)
	require.NoError(t, runner.VerifyDone())
}

func TestHostCodeEnvironment_ReadFileRefInvalid(t *testing.T) {
	workspaceDir := t.TempDir()

	runner := commandrunner.NewDummyCommandRunner()
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", workspaceDir, "config", "--get", "remote.origin.url"},
		},
		Result: commandrunner.Result{Stdout: []byte("https://github.com/example/repo.git")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"clone", "--depth", "1", "https://github.com/example/repo.git", "/home/test/.bentos-labtmp/workspace-1"},
		},
		Result: commandrunner.Result{Stdout: []byte("cloned")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", "/home/test/.bentos-labtmp/workspace-1", "cat-file", "-e", "nope:README.md"},
		},
		Result: commandrunner.Result{
			Stderr: []byte("fatal: Not a valid object name nope"),
		},
		Err: errors.New("exit status 128"),
	})

	logger := &hostTestLogger{}
	env := NewHostCodeEnvironment(HostCodeEnvironmentConfig{
		Runner: runner,
		Getwd: func() (string, error) {
			return workspaceDir, nil
		},
		MakeTempDir: func() (string, error) {
			return "/home/test/.bentos-labtmp/workspace-1", nil
		},
		Logger: logger,
	})

	content, found, err := env.ReadFile(context.Background(), "README.md", "nope")
	require.NoError(t, err)
	require.False(t, found)
	require.Equal(t, "", content)
	require.Len(t, logger.warnLogs, 1)
	require.Contains(t, logger.warnLogs[0], `ReadFile failed for path "README.md" at ref "nope"`)
	require.NoError(t, runner.VerifyDone())
}

func TestHostCodeEnvironment_ReadFileWorkspaceUnexpectedErrorWarnsAndReturnsNotFound(t *testing.T) {
	workspaceDir := t.TempDir()

	runner := commandrunner.NewDummyCommandRunner()
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", workspaceDir, "show", ":README.md"},
		},
		Result: commandrunner.Result{
			Stderr: []byte("fatal: index file corrupt"),
		},
		Err: errors.New("exit status 128"),
	})

	logger := &hostTestLogger{}
	env := NewHostCodeEnvironment(HostCodeEnvironmentConfig{
		Runner: runner,
		Getwd: func() (string, error) {
			return workspaceDir, nil
		},
		Logger: logger,
	})

	content, found, err := env.ReadFile(context.Background(), "README.md", "@staged")
	require.NoError(t, err)
	require.False(t, found)
	require.Equal(t, "", content)
	require.Len(t, logger.warnLogs, 1)
	require.Contains(t, logger.warnLogs[0], `ReadFile failed for path "README.md" at ref "@staged"`)
	require.NoError(t, runner.VerifyDone())
}

func TestHostCodeEnvironment_SetupAgentLocalWorkspaceRefReturnsErrorWhenCheckoutHeadFails(t *testing.T) {
	runner := commandrunner.NewDummyCommandRunner()
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", "/workspace/current", "config", "--get", "remote.origin.url"},
		},
		Result: commandrunner.Result{Stdout: []byte("https://github.com/example/repo.git")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"clone", "--depth", "1", "https://github.com/example/repo.git", "/home/test/.bentos-labtmp/workspace-1"},
		},
		Result: commandrunner.Result{Stdout: []byte("cloned")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", "/home/test/.bentos-labtmp/workspace-1", "rev-parse", "HEAD"},
		},
		Result: commandrunner.Result{Stdout: []byte("head123")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", "/home/test/.bentos-labtmp/workspace-1", "rev-parse", "--verify", "feature/ref^{commit}"},
		},
		Result: commandrunner.Result{Stdout: []byte("abc123")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", "/home/test/.bentos-labtmp/workspace-1", "checkout", "feature/ref"},
		},
		Result: commandrunner.Result{Stderr: []byte("fatal: reference is not a tree")},
		Err:    errors.New("exit status 128"),
	})

	env := NewHostCodeEnvironment(HostCodeEnvironmentConfig{
		Runner: runner,
		Getwd: func() (string, error) {
			return "/workspace/current", nil
		},
		MakeTempDir: func() (string, error) {
			return "/home/test/.bentos-labtmp/workspace-1", nil
		},
	})

	agent, err := env.SetupAgent(context.Background(), domain.CodingAgentSetupOptions{
		Agent: "opencode",
		Ref:   "feature/ref",
	})
	require.Nil(t, agent)
	require.EqualError(t, err, "failed to checkout ref \"feature/ref\": exit status 128: fatal: reference is not a tree")
	require.NoError(t, runner.VerifyDone())
}

func TestHostCodeEnvironment_SetupAgentLocalWorkspaceRefFetchesMissingRef(t *testing.T) {
	workspaceDir := "/workspace/current"
	fetchedRef := localFetchedRefName("feature/ref")
	runner := commandrunner.NewDummyCommandRunner()
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", workspaceDir, "config", "--get", "remote.origin.url"},
		},
		Result: commandrunner.Result{Stdout: []byte("https://github.com/example/repo.git")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"clone", "--depth", "1", "https://github.com/example/repo.git", "/home/test/.bentos-labtmp/workspace-1"},
		},
		Result: commandrunner.Result{Stdout: []byte("cloned")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", "/home/test/.bentos-labtmp/workspace-1", "rev-parse", "HEAD"},
		},
		Result: commandrunner.Result{Stdout: []byte("head123")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", "/home/test/.bentos-labtmp/workspace-1", "rev-parse", "--verify", "feature/ref^{commit}"},
		},
		Result: commandrunner.Result{Stderr: []byte("fatal: Needed a single revision")},
		Err:    errors.New("exit status 128"),
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", "/home/test/.bentos-labtmp/workspace-1", "rev-parse", "--verify", fetchedRef + "^{commit}"},
		},
		Result: commandrunner.Result{Stderr: []byte("fatal: Needed a single revision")},
		Err:    errors.New("exit status 128"),
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", "/home/test/.bentos-labtmp/workspace-1", "rev-parse", "--is-shallow-repository"},
		},
		Result: commandrunner.Result{Stdout: []byte("false")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", "/home/test/.bentos-labtmp/workspace-1", "fetch", "origin", "feature/ref:" + fetchedRef},
		},
		Result: commandrunner.Result{Stdout: []byte("fetched")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", "/home/test/.bentos-labtmp/workspace-1", "rev-parse", "--verify", fetchedRef + "^{commit}"},
		},
		Result: commandrunner.Result{Stdout: []byte("abc123")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", "/home/test/.bentos-labtmp/workspace-1", "checkout", fetchedRef},
		},
		Result: commandrunner.Result{Stdout: []byte("checked out")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", "/home/test/.bentos-labtmp/workspace-1", "rev-parse", "HEAD"},
		},
		Result: commandrunner.Result{Stdout: []byte("head456")},
	})

	env := NewHostCodeEnvironment(HostCodeEnvironmentConfig{
		Runner: runner,
		Getwd: func() (string, error) {
			return workspaceDir, nil
		},
		MakeTempDir: func() (string, error) {
			return "/home/test/.bentos-labtmp/workspace-1", nil
		},
	})

	agent, err := env.SetupAgent(context.Background(), domain.CodingAgentSetupOptions{
		Agent: "opencode",
		Ref:   "feature/ref",
	})
	require.NoError(t, err)
	hostAgent, ok := agent.(*HostOpencodeAgent)
	require.True(t, ok)
	require.Equal(t, "/home/test/.bentos-labtmp/workspace-1", hostAgent.workspaceDir)
	require.NoError(t, runner.VerifyDone())
}

func TestHostCodeEnvironment_SetupAgentRemoteWorkspaceRef(t *testing.T) {
	runner := commandrunner.NewDummyCommandRunner()
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", "/home/test/.bentos-labtmp/workspace-1", "rev-parse", "HEAD"},
		},
		Result: commandrunner.Result{Stdout: []byte("head123")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", "/home/test/.bentos-labtmp/workspace-1", "rev-parse", "--verify", "refs/heads/main^{commit}"},
		},
		Result: commandrunner.Result{Stdout: []byte("abc123")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", "/home/test/.bentos-labtmp/workspace-1", "checkout", "refs/heads/main"},
		},
		Result: commandrunner.Result{Stdout: []byte("checked out")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", "/home/test/.bentos-labtmp/workspace-1", "rev-parse", "HEAD"},
		},
		Result: commandrunner.Result{Stdout: []byte("head456")},
	})

	env := NewHostCodeEnvironment(HostCodeEnvironmentConfig{
		Runner:       runner,
		WorkspaceDir: "/home/test/.bentos-labtmp/workspace-1",
		IsRemote:     true,
	})

	agent, err := env.SetupAgent(context.Background(), domain.CodingAgentSetupOptions{
		Agent: "opencode",
		Ref:   "refs/heads/main",
	})
	require.NoError(t, err)

	hostAgent, ok := agent.(*HostOpencodeAgent)
	require.True(t, ok)
	require.Equal(t, "/home/test/.bentos-labtmp/workspace-1", hostAgent.workspaceDir)
	require.NoError(t, runner.VerifyDone())
}

func TestHostCodeEnvironment_LoadChangedFilesRefsExistWithoutFetchRecovery(t *testing.T) {
	workspaceDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workspaceDir, "main.go"), []byte("package main\n"), 0o644))

	runner := commandrunner.NewDummyCommandRunner()
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", workspaceDir, "config", "--get", "remote.origin.url"},
		},
		Result: commandrunner.Result{Stdout: []byte("https://github.com/example/repo.git")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"clone", "--depth", "1", "https://github.com/example/repo.git", "/home/test/.bentos-labtmp/workspace-1"},
		},
		Result: commandrunner.Result{Stdout: []byte("cloned")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", "/home/test/.bentos-labtmp/workspace-1", "rev-parse", "HEAD"},
		},
		Result: commandrunner.Result{Stdout: []byte("head123")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", "/home/test/.bentos-labtmp/workspace-1", "rev-parse", "--verify", "origin/main^{commit}"},
		},
		Result: commandrunner.Result{Stdout: []byte("abc123")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", "/home/test/.bentos-labtmp/workspace-1", "rev-parse", "--verify", "feature/ref^{commit}"},
		},
		Result: commandrunner.Result{Stdout: []byte("def456")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", "/home/test/.bentos-labtmp/workspace-1", "merge-base", "origin/main", "feature/ref"},
		},
		Result: commandrunner.Result{Stdout: []byte("merge123")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", "/home/test/.bentos-labtmp/workspace-1", "diff", "--name-only", "--diff-filter=ACMRTUXB", "merge123..feature/ref"},
		},
		Result: commandrunner.Result{Stdout: []byte("main.go\n")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", "/home/test/.bentos-labtmp/workspace-1", "show", "feature/ref:main.go"},
		},
		Result: commandrunner.Result{Stdout: []byte("package main\n")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", "/home/test/.bentos-labtmp/workspace-1", "diff", "merge123..feature/ref", "--", "main.go"},
		},
		Result: commandrunner.Result{Stdout: []byte("diff --git a/main.go b/main.go")},
	})

	env := NewHostCodeEnvironment(HostCodeEnvironmentConfig{
		Runner: runner,
		Getwd: func() (string, error) {
			return workspaceDir, nil
		},
		MakeTempDir: func() (string, error) {
			return "/home/test/.bentos-labtmp/workspace-1", nil
		},
	})

	files, err := env.LoadChangedFiles(context.Background(), domain.CodeEnvironmentLoadOptions{
		Base: "origin/main",
		Head: "feature/ref",
	})
	require.NoError(t, err)
	require.Len(t, files, 1)
	require.Equal(t, "main.go", files[0].Path)
	require.NoError(t, runner.VerifyDone())
}

func TestHostCodeEnvironment_LoadChangedFilesFetchesWhenRefMissing(t *testing.T) {
	workspaceDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workspaceDir, "main.go"), []byte("package main\n"), 0o644))
	fetchedRef := localFetchedRefName("feature/ref")

	runner := commandrunner.NewDummyCommandRunner()
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", workspaceDir, "config", "--get", "remote.origin.url"},
		},
		Result: commandrunner.Result{Stdout: []byte("https://github.com/example/repo.git")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"clone", "--depth", "1", "https://github.com/example/repo.git", "/home/test/.bentos-labtmp/workspace-1"},
		},
		Result: commandrunner.Result{Stdout: []byte("cloned")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", "/home/test/.bentos-labtmp/workspace-1", "rev-parse", "HEAD"},
		},
		Result: commandrunner.Result{Stdout: []byte("head123")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", "/home/test/.bentos-labtmp/workspace-1", "rev-parse", "--verify", "origin/main^{commit}"},
		},
		Result: commandrunner.Result{Stdout: []byte("abc123")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", "/home/test/.bentos-labtmp/workspace-1", "rev-parse", "--verify", "feature/ref^{commit}"},
		},
		Result: commandrunner.Result{Stderr: []byte("fatal: Needed a single revision")},
		Err:    errors.New("exit status 128"),
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", "/home/test/.bentos-labtmp/workspace-1", "rev-parse", "--verify", fetchedRef + "^{commit}"},
		},
		Result: commandrunner.Result{Stderr: []byte("fatal: Needed a single revision")},
		Err:    errors.New("exit status 128"),
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", "/home/test/.bentos-labtmp/workspace-1", "rev-parse", "--is-shallow-repository"},
		},
		Result: commandrunner.Result{Stdout: []byte("false")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", "/home/test/.bentos-labtmp/workspace-1", "fetch", "origin", "feature/ref:" + fetchedRef},
		},
		Result: commandrunner.Result{Stdout: []byte("fetched")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", "/home/test/.bentos-labtmp/workspace-1", "rev-parse", "--verify", fetchedRef + "^{commit}"},
		},
		Result: commandrunner.Result{Stdout: []byte("def456")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", "/home/test/.bentos-labtmp/workspace-1", "merge-base", "origin/main", fetchedRef},
		},
		Result: commandrunner.Result{Stdout: []byte("merge123")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", "/home/test/.bentos-labtmp/workspace-1", "diff", "--name-only", "--diff-filter=ACMRTUXB", "merge123.." + fetchedRef},
		},
		Result: commandrunner.Result{Stdout: []byte("main.go\n")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", "/home/test/.bentos-labtmp/workspace-1", "show", fetchedRef + ":main.go"},
		},
		Result: commandrunner.Result{Stdout: []byte("package main\n")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", "/home/test/.bentos-labtmp/workspace-1", "diff", "merge123.." + fetchedRef, "--", "main.go"},
		},
		Result: commandrunner.Result{Stdout: []byte("diff --git a/main.go b/main.go")},
	})

	env := NewHostCodeEnvironment(HostCodeEnvironmentConfig{
		Runner: runner,
		Getwd: func() (string, error) {
			return workspaceDir, nil
		},
		MakeTempDir: func() (string, error) {
			return "/home/test/.bentos-labtmp/workspace-1", nil
		},
	})

	files, err := env.LoadChangedFiles(context.Background(), domain.CodeEnvironmentLoadOptions{
		Base: "origin/main",
		Head: "feature/ref",
	})
	require.NoError(t, err)
	require.Len(t, files, 1)
	require.NoError(t, runner.VerifyDone())
}

func TestHostCodeEnvironment_LoadChangedFilesReturnsErrorWhenRefStillMissingAfterFetch(t *testing.T) {
	workspaceDir := t.TempDir()
	fetchedRef := localFetchedRefName("feature/ref")
	runner := commandrunner.NewDummyCommandRunner()
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", workspaceDir, "config", "--get", "remote.origin.url"},
		},
		Result: commandrunner.Result{Stdout: []byte("https://github.com/example/repo.git")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"clone", "--depth", "1", "https://github.com/example/repo.git", "/home/test/.bentos-labtmp/workspace-1"},
		},
		Result: commandrunner.Result{Stdout: []byte("cloned")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", "/home/test/.bentos-labtmp/workspace-1", "rev-parse", "HEAD"},
		},
		Result: commandrunner.Result{Stdout: []byte("head123")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", "/home/test/.bentos-labtmp/workspace-1", "rev-parse", "--verify", "HEAD^{commit}"},
		},
		Result: commandrunner.Result{Stdout: []byte("head123")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", "/home/test/.bentos-labtmp/workspace-1", "rev-parse", "--verify", "feature/ref^{commit}"},
		},
		Result: commandrunner.Result{Stderr: []byte("fatal: Needed a single revision")},
		Err:    errors.New("exit status 128"),
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", "/home/test/.bentos-labtmp/workspace-1", "rev-parse", "--verify", fetchedRef + "^{commit}"},
		},
		Result: commandrunner.Result{Stderr: []byte("fatal: Needed a single revision")},
		Err:    errors.New("exit status 128"),
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", "/home/test/.bentos-labtmp/workspace-1", "rev-parse", "--is-shallow-repository"},
		},
		Result: commandrunner.Result{Stdout: []byte("false")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", "/home/test/.bentos-labtmp/workspace-1", "fetch", "origin", "feature/ref:" + fetchedRef},
		},
		Result: commandrunner.Result{Stderr: []byte("fatal: couldn't find remote ref feature/ref")},
		Err:    errors.New("exit status 128"),
	})

	env := NewHostCodeEnvironment(HostCodeEnvironmentConfig{
		Runner: runner,
		Getwd: func() (string, error) {
			return workspaceDir, nil
		},
		MakeTempDir: func() (string, error) {
			return "/home/test/.bentos-labtmp/workspace-1", nil
		},
	})

	_, err := env.LoadChangedFiles(context.Background(), domain.CodeEnvironmentLoadOptions{
		Head: "feature/ref",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), `failed to resolve ref "feature/ref" in local workspace`)
	require.NoError(t, runner.VerifyDone())
}

func TestHostCodeEnvironment_LoadChangedFilesTokenModesSkipRefVerification(t *testing.T) {
	stagedWorkspace := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(stagedWorkspace, "main.go"), []byte("package main\n"), 0o644))

	stagedRunner := commandrunner.NewDummyCommandRunner()
	stagedRunner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", stagedWorkspace, "rev-parse", "HEAD"},
		},
		Result: commandrunner.Result{Stdout: []byte("head123")},
	})
	stagedRunner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", stagedWorkspace, "diff", "--cached", "--name-only", "--diff-filter=ACMRTUXB"},
		},
		Result: commandrunner.Result{Stdout: []byte("main.go\n")},
	})
	stagedRunner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", stagedWorkspace, "diff", "--cached", "--", "main.go"},
		},
		Result: commandrunner.Result{Stdout: []byte("diff --git a/main.go b/main.go")},
	})

	stagedEnv := NewHostCodeEnvironment(HostCodeEnvironmentConfig{
		Runner: stagedRunner,
		Getwd: func() (string, error) {
			return stagedWorkspace, nil
		},
	})
	_, err := stagedEnv.LoadChangedFiles(context.Background(), domain.CodeEnvironmentLoadOptions{
		Head: "@staged",
	})
	require.NoError(t, err)
	require.NoError(t, stagedRunner.VerifyDone())
	for _, call := range stagedRunner.Calls() {
		require.NotContains(t, strings.Join(call.Args, " "), "rev-parse --verify")
		require.NotEqual(t, []string{"-C", stagedWorkspace, "fetch", "--all", "--prune"}, call.Args)
	}

	allWorkspace := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(allWorkspace, "main.go"), []byte("package main\n"), 0o644))

	allRunner := commandrunner.NewDummyCommandRunner()
	allRunner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", allWorkspace, "rev-parse", "HEAD"},
		},
		Result: commandrunner.Result{Stdout: []byte("head123")},
	})
	allRunner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", allWorkspace, "diff", "--cached", "--name-only", "--diff-filter=ACMRTUXB"},
		},
		Result: commandrunner.Result{Stdout: []byte("main.go\n")},
	})
	allRunner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", allWorkspace, "diff", "--name-only", "--diff-filter=ACMRTUXB"},
		},
		Result: commandrunner.Result{Stdout: []byte("")},
	})
	allRunner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", allWorkspace, "ls-files", "--others", "--exclude-standard"},
		},
		Result: commandrunner.Result{Stdout: []byte("")},
	})
	allRunner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", allWorkspace, "diff", "--cached", "--", "main.go"},
		},
		Result: commandrunner.Result{Stdout: []byte("diff --git a/main.go b/main.go")},
	})
	allRunner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", allWorkspace, "diff", "--", "main.go"},
		},
		Result: commandrunner.Result{Stdout: []byte("")},
	})

	allEnv := NewHostCodeEnvironment(HostCodeEnvironmentConfig{
		Runner: allRunner,
		Getwd: func() (string, error) {
			return allWorkspace, nil
		},
	})
	_, err = allEnv.LoadChangedFiles(context.Background(), domain.CodeEnvironmentLoadOptions{
		Head: "@all",
	})
	require.NoError(t, err)
	require.NoError(t, allRunner.VerifyDone())
	for _, call := range allRunner.Calls() {
		require.NotContains(t, strings.Join(call.Args, " "), "rev-parse --verify")
		require.NotEqual(t, []string{"-C", allWorkspace, "fetch", "--all", "--prune"}, call.Args)
	}
}

func TestNewHostCodeEnvironmentTempDirMakerCreatesUniqueDirsUnderBentosLabTmp(t *testing.T) {
	homeDir := t.TempDir()
	makeTempDir := newHostCodeEnvironmentTempDirMaker(func() (string, error) {
		return homeDir, nil
	})

	first, err := makeTempDir()
	require.NoError(t, err)
	second, err := makeTempDir()
	require.NoError(t, err)
	require.NotEqual(t, first, second)

	baseDir := filepath.Join(homeDir, hostCodeEnvironmentTempBaseDirName)
	require.Equal(t, baseDir, filepath.Dir(first))
	require.Equal(t, baseDir, filepath.Dir(second))
	require.True(t, strings.HasPrefix(filepath.Base(first), "bentos-coding-agent-"))
	require.True(t, strings.HasPrefix(filepath.Base(second), "bentos-coding-agent-"))

	info, err := os.Stat(baseDir)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o700), info.Mode().Perm())
}

func TestNewHostCodeEnvironmentTempDirMakerFailsWhenHomeDirectoryCannotBeResolved(t *testing.T) {
	makeTempDir := newHostCodeEnvironmentTempDirMaker(func() (string, error) {
		return "", errors.New("home lookup failed")
	})

	_, err := makeTempDir()
	require.EqualError(t, err, "failed to resolve user home directory: home lookup failed")
}

func TestNewHostCodeEnvironmentTempDirMakerFailsWhenHomeDirectoryIsEmpty(t *testing.T) {
	makeTempDir := newHostCodeEnvironmentTempDirMaker(func() (string, error) {
		return "   ", nil
	})

	_, err := makeTempDir()
	require.EqualError(t, err, "failed to resolve user home directory: empty path")
}

func TestNewHostCodeEnvironmentTempDirMakerFailsWhenBaseDirCannotBeCreated(t *testing.T) {
	rootFilePath := filepath.Join(t.TempDir(), "root-file")
	require.NoError(t, os.WriteFile(rootFilePath, []byte("content"), 0o644))

	makeTempDir := newHostCodeEnvironmentTempDirMaker(func() (string, error) {
		return rootFilePath, nil
	})

	_, err := makeTempDir()
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to create temporary workspace base directory")
}

func TestHostCodeEnvironment_PrepareWorkspaceRemoteLogsTmpFolderAtDebug(t *testing.T) {
	runner := commandrunner.NewDummyCommandRunner()
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"clone", "--depth", "1", "https://github.com/example/repo.git", "/home/test/.bentos-labtmp/workspace-1"},
		},
		Result: commandrunner.Result{Stdout: []byte("cloned")},
	})
	logger := &hostTestLogger{}
	env := NewHostCodeEnvironment(HostCodeEnvironmentConfig{
		Runner: runner,
		MakeTempDir: func() (string, error) {
			return "/home/test/.bentos-labtmp/workspace-1", nil
		},
		Logger: logger,
	})

	err := env.prepareWorkspace(context.Background(), "https://github.com/example/repo.git")
	require.NoError(t, err)
	require.NoError(t, runner.VerifyDone())
	require.Len(t, logger.debugLogs, 2)
	require.Contains(t, logger.debugLogs[0], `Code environment temporary workspace directory is "/home/test/.bentos-labtmp/workspace-1"`)
	require.Contains(t, logger.debugLogs[0], `under tmp folder "/home/test/.bentos-labtmp"`)
	require.Contains(t, logger.debugLogs[1], `Cloned repo to /home/test/.bentos-labtmp/workspace-1 (shallow=true)`)
}

func TestHostCodeEnvironment_SetupAgentPropagatesLoggerToHostAgent(t *testing.T) {
	logger := &hostTestLogger{}
	env := NewHostCodeEnvironment(HostCodeEnvironmentConfig{
		Runner: commandrunner.NewDummyCommandRunner(),
		Getwd: func() (string, error) {
			return "/workspace/current", nil
		},
		Logger: logger,
	})

	agent, err := env.SetupAgent(context.Background(), domain.CodingAgentSetupOptions{
		Agent: "opencode",
	})
	require.NoError(t, err)

	hostAgent, ok := agent.(*HostOpencodeAgent)
	require.True(t, ok)
	require.Same(t, logger, hostAgent.logger)
}

func TestHostCodeEnvironment_CleanupRemoteWorkspaceRemovesDir(t *testing.T) {
	tempDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "tmp.txt"), []byte("data"), 0o644))

	env := NewHostCodeEnvironment(HostCodeEnvironmentConfig{
		Runner: commandrunner.NewDummyCommandRunner(),
	})
	env.workspaceDir = tempDir
	env.isRemote = true

	err := env.Cleanup(context.Background())
	require.NoError(t, err)
	_, statErr := os.Stat(tempDir)
	require.True(t, os.IsNotExist(statErr))
}

func TestHostCodeEnvironment_CleanupLocalWorkspaceNoop(t *testing.T) {
	tempDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "tmp.txt"), []byte("data"), 0o644))

	env := NewHostCodeEnvironment(HostCodeEnvironmentConfig{
		Runner: commandrunner.NewDummyCommandRunner(),
	})
	env.workspaceDir = tempDir
	env.isRemote = false

	err := env.Cleanup(context.Background())
	require.NoError(t, err)
	_, statErr := os.Stat(tempDir)
	require.NoError(t, statErr)
}

func TestHostCodeEnvironment_PushChangesSkipsWhenNoChanges(t *testing.T) {
	runner := commandrunner.NewDummyCommandRunner()
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{Name: "git", Args: []string{"-C", "/workspace", "status", "--porcelain"}},
		Result:   commandrunner.Result{Stdout: []byte("\n")},
	})

	env := NewHostCodeEnvironment(HostCodeEnvironmentConfig{
		Runner: runner,
	})
	env.workspaceDir = "/workspace"

	result, err := env.PushChanges(context.Background(), domain.CodeEnvironmentPushOptions{
		TargetBranch:  "feature",
		CommitMessage: "autogen: add tests/docs/comments",
	})
	require.NoError(t, err)
	require.False(t, result.Pushed)
	require.NoError(t, runner.VerifyDone())
}

func TestHostCodeEnvironment_PushChangesCommitsAndPushes(t *testing.T) {
	runner := commandrunner.NewDummyCommandRunner()
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{Name: "git", Args: []string{"-C", "/workspace", "status", "--porcelain"}},
		Result:   commandrunner.Result{Stdout: []byte(" M foo.go\n")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{Name: "git", Args: []string{"-C", "/workspace", "add", "-A"}},
		Result:   commandrunner.Result{Stdout: []byte("ok")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{Name: "git", Args: []string{"-C", "/workspace", "commit", "-m", "autogen: add tests/docs/comments"}},
		Result:   commandrunner.Result{Stdout: []byte("ok")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{Name: "git", Args: []string{"-C", "/workspace", "push", "origin", "HEAD:feature"}},
		Result:   commandrunner.Result{Stdout: []byte("ok")},
	})

	env := NewHostCodeEnvironment(HostCodeEnvironmentConfig{
		Runner: runner,
	})
	env.workspaceDir = "/workspace"

	result, err := env.PushChanges(context.Background(), domain.CodeEnvironmentPushOptions{
		TargetBranch:  "feature",
		CommitMessage: "autogen: add tests/docs/comments",
	})
	require.NoError(t, err)
	require.True(t, result.Pushed)
	require.NoError(t, runner.VerifyDone())
}

type hostTestLogger struct {
	debugLogs []string
	warnLogs  []string
}

func (l *hostTestLogger) Tracef(_ string, _ ...any) {}

func (l *hostTestLogger) Debugf(format string, args ...any) {
	l.debugLogs = append(l.debugLogs, fmt.Sprintf(format, args...))
}

func (l *hostTestLogger) Infof(_ string, _ ...any) {}

func (l *hostTestLogger) Warnf(format string, args ...any) {
	l.warnLogs = append(l.warnLogs, fmt.Sprintf(format, args...))
}

func (l *hostTestLogger) Errorf(_ string, _ ...any) {}
