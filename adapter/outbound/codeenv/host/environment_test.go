package host

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"bentos-backend/adapter/outbound/commandrunner"
	"bentos-backend/domain"

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
			return "/home/test/.sisutmp/workspace-1", nil
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
		WorkspaceDir: "/home/test/.sisutmp/workspace-1",
		IsRemote:     true,
	})

	agent, err := env.SetupAgent(context.Background(), domain.CodingAgentSetupOptions{
		Agent: "OpenCode",
	})
	require.NoError(t, err)

	hostAgent, ok := agent.(*HostOpencodeAgent)
	require.True(t, ok)
	require.Equal(t, "/home/test/.sisutmp/workspace-1", hostAgent.workspaceDir)
	require.NoError(t, runner.VerifyDone())
}

func TestHostCodeEnvironment_SetupAgentLocalWorkspaceRef(t *testing.T) {
	runner := commandrunner.NewDummyCommandRunner()
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", "/workspace/current", "rev-parse", "HEAD"},
		},
		Result: commandrunner.Result{Stdout: []byte("head123")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", "/workspace/current", "rev-parse", "--verify", "feature/ref^{commit}"},
		},
		Result: commandrunner.Result{Stdout: []byte("abc123")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", "/workspace/current", "checkout", "feature/ref"},
		},
		Result: commandrunner.Result{Stdout: []byte("checked out")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", "/workspace/current", "rev-parse", "HEAD"},
		},
		Result: commandrunner.Result{Stdout: []byte("head456")},
	})

	env := NewHostCodeEnvironment(HostCodeEnvironmentConfig{
		Runner: runner,
		Getwd: func() (string, error) {
			return "/workspace/current", nil
		},
	})

	agent, err := env.SetupAgent(context.Background(), domain.CodingAgentSetupOptions{
		Agent: "opencode",
		Ref:   "feature/ref",
	})
	require.NoError(t, err)

	hostAgent, ok := agent.(*HostOpencodeAgent)
	require.True(t, ok)
	require.Equal(t, "/workspace/current", hostAgent.workspaceDir)
	require.NoError(t, runner.VerifyDone())
}

func TestHostCodeEnvironment_SetupAgentLocalWorkspaceRefReturnsErrorWhenCheckoutHeadFails(t *testing.T) {
	runner := commandrunner.NewDummyCommandRunner()
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", "/workspace/current", "rev-parse", "HEAD"},
		},
		Result: commandrunner.Result{Stdout: []byte("head123")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", "/workspace/current", "rev-parse", "--verify", "feature/ref^{commit}"},
		},
		Result: commandrunner.Result{Stdout: []byte("abc123")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", "/workspace/current", "checkout", "feature/ref"},
		},
		Result: commandrunner.Result{Stderr: []byte("fatal: reference is not a tree")},
		Err:    errors.New("exit status 128"),
	})

	env := NewHostCodeEnvironment(HostCodeEnvironmentConfig{
		Runner: runner,
		Getwd: func() (string, error) {
			return "/workspace/current", nil
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
			Args: []string{"-C", workspaceDir, "rev-parse", "HEAD"},
		},
		Result: commandrunner.Result{Stdout: []byte("head123")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", workspaceDir, "rev-parse", "--verify", "feature/ref^{commit}"},
		},
		Result: commandrunner.Result{Stderr: []byte("fatal: Needed a single revision")},
		Err:    errors.New("exit status 128"),
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", workspaceDir, "rev-parse", "--verify", fetchedRef + "^{commit}"},
		},
		Result: commandrunner.Result{Stderr: []byte("fatal: Needed a single revision")},
		Err:    errors.New("exit status 128"),
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", workspaceDir, "fetch", "--unshallow", "origin", "feature/ref:" + fetchedRef},
		},
		Result: commandrunner.Result{Stdout: []byte("fetched")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", workspaceDir, "rev-parse", "--verify", fetchedRef + "^{commit}"},
		},
		Result: commandrunner.Result{Stdout: []byte("abc123")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", workspaceDir, "checkout", fetchedRef},
		},
		Result: commandrunner.Result{Stdout: []byte("checked out")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", workspaceDir, "rev-parse", "HEAD"},
		},
		Result: commandrunner.Result{Stdout: []byte("head456")},
	})

	env := NewHostCodeEnvironment(HostCodeEnvironmentConfig{
		Runner: runner,
		Getwd: func() (string, error) {
			return workspaceDir, nil
		},
	})

	agent, err := env.SetupAgent(context.Background(), domain.CodingAgentSetupOptions{
		Agent: "opencode",
		Ref:   "feature/ref",
	})
	require.NoError(t, err)
	hostAgent, ok := agent.(*HostOpencodeAgent)
	require.True(t, ok)
	require.Equal(t, workspaceDir, hostAgent.workspaceDir)
	require.NoError(t, runner.VerifyDone())
}

func TestHostCodeEnvironment_SetupAgentRemoteWorkspaceRef(t *testing.T) {
	runner := commandrunner.NewDummyCommandRunner()
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", "/home/test/.sisutmp/workspace-1", "rev-parse", "HEAD"},
		},
		Result: commandrunner.Result{Stdout: []byte("head123")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", "/home/test/.sisutmp/workspace-1", "rev-parse", "--verify", "refs/heads/main^{commit}"},
		},
		Result: commandrunner.Result{Stdout: []byte("abc123")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", "/home/test/.sisutmp/workspace-1", "checkout", "refs/heads/main"},
		},
		Result: commandrunner.Result{Stdout: []byte("checked out")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", "/home/test/.sisutmp/workspace-1", "rev-parse", "HEAD"},
		},
		Result: commandrunner.Result{Stdout: []byte("head456")},
	})

	env := NewHostCodeEnvironment(HostCodeEnvironmentConfig{
		Runner:       runner,
		WorkspaceDir: "/home/test/.sisutmp/workspace-1",
		IsRemote:     true,
	})

	agent, err := env.SetupAgent(context.Background(), domain.CodingAgentSetupOptions{
		Agent: "opencode",
		Ref:   "refs/heads/main",
	})
	require.NoError(t, err)

	hostAgent, ok := agent.(*HostOpencodeAgent)
	require.True(t, ok)
	require.Equal(t, "/home/test/.sisutmp/workspace-1", hostAgent.workspaceDir)
	require.NoError(t, runner.VerifyDone())
}

func TestHostCodeEnvironment_LoadChangedFilesRefsExistWithoutFetchRecovery(t *testing.T) {
	workspaceDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workspaceDir, "main.go"), []byte("package main\n"), 0o644))

	runner := commandrunner.NewDummyCommandRunner()
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", workspaceDir, "rev-parse", "HEAD"},
		},
		Result: commandrunner.Result{Stdout: []byte("head123")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", workspaceDir, "rev-parse", "--verify", "origin/main^{commit}"},
		},
		Result: commandrunner.Result{Stdout: []byte("abc123")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", workspaceDir, "rev-parse", "--verify", "feature/ref^{commit}"},
		},
		Result: commandrunner.Result{Stdout: []byte("def456")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", workspaceDir, "merge-base", "origin/main", "feature/ref"},
		},
		Result: commandrunner.Result{Stdout: []byte("merge123")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", workspaceDir, "diff", "--name-only", "--diff-filter=ACMRTUXB", "merge123..feature/ref"},
		},
		Result: commandrunner.Result{Stdout: []byte("main.go\n")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", workspaceDir, "show", "feature/ref:main.go"},
		},
		Result: commandrunner.Result{Stdout: []byte("package main\n")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", workspaceDir, "diff", "merge123..feature/ref", "--", "main.go"},
		},
		Result: commandrunner.Result{Stdout: []byte("diff --git a/main.go b/main.go")},
	})

	env := NewHostCodeEnvironment(HostCodeEnvironmentConfig{
		Runner: runner,
		Getwd: func() (string, error) {
			return workspaceDir, nil
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
			Args: []string{"-C", workspaceDir, "rev-parse", "HEAD"},
		},
		Result: commandrunner.Result{Stdout: []byte("head123")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", workspaceDir, "rev-parse", "--verify", "origin/main^{commit}"},
		},
		Result: commandrunner.Result{Stdout: []byte("abc123")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", workspaceDir, "rev-parse", "--verify", "feature/ref^{commit}"},
		},
		Result: commandrunner.Result{Stderr: []byte("fatal: Needed a single revision")},
		Err:    errors.New("exit status 128"),
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", workspaceDir, "rev-parse", "--verify", fetchedRef + "^{commit}"},
		},
		Result: commandrunner.Result{Stderr: []byte("fatal: Needed a single revision")},
		Err:    errors.New("exit status 128"),
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", workspaceDir, "fetch", "--unshallow", "origin", "feature/ref:" + fetchedRef},
		},
		Result: commandrunner.Result{Stdout: []byte("fetched")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", workspaceDir, "rev-parse", "--verify", fetchedRef + "^{commit}"},
		},
		Result: commandrunner.Result{Stdout: []byte("def456")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", workspaceDir, "merge-base", "origin/main", fetchedRef},
		},
		Result: commandrunner.Result{Stdout: []byte("merge123")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", workspaceDir, "diff", "--name-only", "--diff-filter=ACMRTUXB", "merge123.." + fetchedRef},
		},
		Result: commandrunner.Result{Stdout: []byte("main.go\n")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", workspaceDir, "show", fetchedRef + ":main.go"},
		},
		Result: commandrunner.Result{Stdout: []byte("package main\n")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", workspaceDir, "diff", "merge123.." + fetchedRef, "--", "main.go"},
		},
		Result: commandrunner.Result{Stdout: []byte("diff --git a/main.go b/main.go")},
	})

	env := NewHostCodeEnvironment(HostCodeEnvironmentConfig{
		Runner: runner,
		Getwd: func() (string, error) {
			return workspaceDir, nil
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
			Args: []string{"-C", workspaceDir, "rev-parse", "HEAD"},
		},
		Result: commandrunner.Result{Stdout: []byte("head123")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", workspaceDir, "rev-parse", "--verify", "HEAD^{commit}"},
		},
		Result: commandrunner.Result{Stdout: []byte("head123")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", workspaceDir, "rev-parse", "--verify", "feature/ref^{commit}"},
		},
		Result: commandrunner.Result{Stderr: []byte("fatal: Needed a single revision")},
		Err:    errors.New("exit status 128"),
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", workspaceDir, "rev-parse", "--verify", fetchedRef + "^{commit}"},
		},
		Result: commandrunner.Result{Stderr: []byte("fatal: Needed a single revision")},
		Err:    errors.New("exit status 128"),
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "git",
			Args: []string{"-C", workspaceDir, "fetch", "--unshallow", "origin", "feature/ref:" + fetchedRef},
		},
		Result: commandrunner.Result{Stderr: []byte("fatal: couldn't find remote ref feature/ref")},
		Err:    errors.New("exit status 128"),
	})

	env := NewHostCodeEnvironment(HostCodeEnvironmentConfig{
		Runner: runner,
		Getwd: func() (string, error) {
			return workspaceDir, nil
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

func TestNewHostCodeEnvironmentTempDirMakerCreatesUniqueDirsUnderSisuTmp(t *testing.T) {
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
			Args: []string{"clone", "--depth", "1", "https://github.com/example/repo.git", "/home/test/.sisutmp/workspace-1"},
		},
		Result: commandrunner.Result{Stdout: []byte("cloned")},
	})
	logger := &hostTestLogger{}
	env := NewHostCodeEnvironment(HostCodeEnvironmentConfig{
		Runner: runner,
		MakeTempDir: func() (string, error) {
			return "/home/test/.sisutmp/workspace-1", nil
		},
		Logger: logger,
	})

	err := env.prepareWorkspace(context.Background(), "https://github.com/example/repo.git")
	require.NoError(t, err)
	require.NoError(t, runner.VerifyDone())
	require.Len(t, logger.debugLogs, 2)
	require.Contains(t, logger.debugLogs[0], `Code environment temporary workspace directory is "/home/test/.sisutmp/workspace-1"`)
	require.Contains(t, logger.debugLogs[0], `under tmp folder "/home/test/.sisutmp"`)
	require.Contains(t, logger.debugLogs[1], `Cloned repo to /home/test/.sisutmp/workspace-1 (shallow=true)`)
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

type hostTestLogger struct {
	debugLogs []string
}

func (l *hostTestLogger) Tracef(_ string, _ ...any) {}

func (l *hostTestLogger) Debugf(format string, args ...any) {
	l.debugLogs = append(l.debugLogs, fmt.Sprintf(format, args...))
}

func (l *hostTestLogger) Infof(_ string, _ ...any) {}

func (l *hostTestLogger) Warnf(_ string, _ ...any) {}

func (l *hostTestLogger) Errorf(_ string, _ ...any) {}
