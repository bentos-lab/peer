package host

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"bentos-backend/adapter/outbound/commandrunner"
	"bentos-backend/domain"
	"bentos-backend/usecase"
	"bentos-backend/usecase/contracts"
)

// HostCodeEnvironmentConfig contains dependencies for host environment setup.
type HostCodeEnvironmentConfig struct {
	Runner       commandrunner.Runner
	AgentRunner  commandrunner.StreamRunner
	Getwd        func() (string, error)
	MakeTempDir  func() (string, error)
	Logger       usecase.Logger
	WorkspaceDir string
	IsRemote     bool
}

// HostCodeEnvironment prepares code operations that run directly on the host machine.
type HostCodeEnvironment struct {
	runner       commandrunner.Runner
	agentRunner  commandrunner.StreamRunner
	getwd        func() (string, error)
	makeTempDir  func() (string, error)
	logger       usecase.Logger
	workspaceDir string
	isRemote     bool
	mu           sync.Mutex
}

const hostCodeEnvironmentTempBaseDirName = ".sisutmp"
const hostCodeEnvironmentFetchedRefPrefix = "refs/bentos/fetched/"

// NewHostCodeEnvironment creates a host environment with injected dependencies.
func NewHostCodeEnvironment(cfg HostCodeEnvironmentConfig) *HostCodeEnvironment {
	defaults := resolveHostDefaults(
		cfg.Runner,
		cfg.AgentRunner,
		cfg.Getwd,
		cfg.MakeTempDir,
		cfg.Logger,
	)
	return &HostCodeEnvironment{
		runner:       defaults.runner,
		agentRunner:  defaults.agentRunner,
		getwd:        defaults.getwd,
		makeTempDir:  defaults.makeTempDir,
		logger:       defaults.logger,
		workspaceDir: strings.TrimSpace(cfg.WorkspaceDir),
		isRemote:     cfg.IsRemote,
	}
}

// SetupAgent checks out the requested head ref and returns a host-backed coding agent.
func (e *HostCodeEnvironment) SetupAgent(ctx context.Context, opts domain.CodingAgentSetupOptions) (contracts.CodingAgent, error) {
	agentName := strings.ToLower(strings.TrimSpace(opts.Agent))
	if agentName == "" {
		return nil, fmt.Errorf("agent is required")
	}

	workspaceDir, err := e.workspaceDirForRun()
	if err != nil {
		return nil, err
	}

	headRef := strings.TrimSpace(opts.Ref)
	if e.isRemote && isWorkspaceTokenRef(headRef) && headRef != "" {
		return nil, fmt.Errorf("ref %q requires local workspace mode", headRef)
	}
	if err := e.syncRef(ctx, workspaceDir, headRef); err != nil {
		return nil, err
	}

	switch agentName {
	case "opencode":
		return NewHostOpencodeAgent(workspaceDir, e.agentRunner, e.logger), nil
	default:
		return nil, fmt.Errorf("unsupported coding agent: %s", agentName)
	}
}

// LoadChangedFiles resolves changed files from the selected comparison mode.
func (e *HostCodeEnvironment) LoadChangedFiles(ctx context.Context, opts domain.CodeEnvironmentLoadOptions) ([]domain.ChangedFile, error) {
	workspaceDir, err := e.workspaceDirForRun()
	if err != nil {
		return nil, err
	}

	currentHead, headErr := e.getCurrentHead(ctx, workspaceDir)
	if headErr != nil {
		e.logger.Debugf("Failed to get current HEAD: %v", headErr)
	} else {
		e.logger.Debugf("Current HEAD in workspace: %s", currentHead)
	}

	base := strings.TrimSpace(opts.Base)
	head := strings.TrimSpace(opts.Head)
	resolvedBase, resolvedHead, mergeBase, err := e.resolveDiffRefs(ctx, workspaceDir, base, head)
	if err != nil {
		return nil, err
	}
	if !isWorkspaceTokenRef(head) {
		e.logger.Debugf("LoadChangedFiles: base=%s (resolved=%s), head=%s (resolved=%s), mergeBase=%s, workspaceDir=%s", base, resolvedBase, head, resolvedHead, mergeBase, workspaceDir)
	}

	paths, err := e.collectChangedPaths(ctx, workspaceDir, head, mergeBase, resolvedHead)
	if err != nil {
		return nil, err
	}

	files := make([]domain.ChangedFile, 0, len(paths))
	for _, path := range paths {
		content, readErr := e.readPathContent(ctx, workspaceDir, path, resolvedHead)
		if readErr != nil {
			return nil, readErr
		}

		diffSnippet, diffErr := e.diffForPath(ctx, workspaceDir, path, mergeBase, resolvedHead)
		if diffErr != nil {
			return nil, diffErr
		}
		if strings.TrimSpace(diffSnippet) == "" && e.isRemote {
			// Remote workspaces with token heads can naturally produce no local changes.
			continue
		}
		files = append(files, domain.ChangedFile{
			Path:        path,
			Content:     content,
			DiffSnippet: diffSnippet,
		})
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("%w: no changes found for base %q and head %q", domain.ErrNoCodeChanges, base, head)
	}
	return files, nil
}

// ReadFile reads a repository-relative file at the provided ref.
func (e *HostCodeEnvironment) ReadFile(ctx context.Context, path string, ref string) (string, bool, error) {
	workspaceDir, err := e.workspaceDirForRun()
	if err != nil {
		return "", false, err
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return "", false, fmt.Errorf("path is required")
	}
	ref = strings.TrimSpace(ref)

	if isWorkspaceTokenRef(ref) {
		content, found, err := e.readWorkspaceFile(ctx, workspaceDir, path)
		if err != nil {
			return "", false, err
		}
		return content, found, nil
	}

	content, found, err := e.readRefFile(ctx, workspaceDir, path, ref)
	if err != nil {
		return "", false, err
	}
	return content, found, nil
}

// Cleanup removes any temporary workspace created for remote repositories.
func (e *HostCodeEnvironment) Cleanup(_ context.Context) error {
	if !e.isRemote {
		return nil
	}
	workspaceDir := strings.TrimSpace(e.workspaceDir)
	if workspaceDir == "" {
		return nil
	}
	return os.RemoveAll(workspaceDir)
}

// WorkspaceDir returns the active workspace directory, if any.
func (e *HostCodeEnvironment) WorkspaceDir() string {
	return strings.TrimSpace(e.workspaceDir)
}

// PushChanges commits and pushes workspace changes to the target branch.
func (e *HostCodeEnvironment) PushChanges(ctx context.Context, opts domain.CodeEnvironmentPushOptions) (domain.CodeEnvironmentPushResult, error) {
	workspaceDir, err := e.workspaceDirForRun()
	if err != nil {
		return domain.CodeEnvironmentPushResult{}, err
	}

	targetBranch := strings.TrimSpace(opts.TargetBranch)
	if targetBranch == "" {
		return domain.CodeEnvironmentPushResult{}, fmt.Errorf("target branch is required")
	}
	commitMessage := strings.TrimSpace(opts.CommitMessage)
	if commitMessage == "" {
		return domain.CodeEnvironmentPushResult{}, fmt.Errorf("commit message is required")
	}
	remoteName := strings.TrimSpace(opts.RemoteName)
	if remoteName == "" {
		remoteName = "origin"
	}

	status, err := e.git(ctx, workspaceDir, "status", "--porcelain")
	if err != nil {
		return domain.CodeEnvironmentPushResult{}, err
	}
	if strings.TrimSpace(string(status.Stdout)) == "" {
		e.logger.Infof("No autogen changes detected; skipping commit and push.")
		return domain.CodeEnvironmentPushResult{Pushed: false}, nil
	}

	if _, err := e.git(ctx, workspaceDir, "add", "-A"); err != nil {
		return domain.CodeEnvironmentPushResult{}, err
	}
	if _, err := e.git(ctx, workspaceDir, "commit", "-m", commitMessage); err != nil {
		return domain.CodeEnvironmentPushResult{}, err
	}
	if _, err := e.git(ctx, workspaceDir, "push", remoteName, fmt.Sprintf("HEAD:%s", targetBranch)); err != nil {
		return domain.CodeEnvironmentPushResult{}, err
	}

	return domain.CodeEnvironmentPushResult{Pushed: true}, nil
}
