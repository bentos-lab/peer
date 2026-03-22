package host

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/bentos-lab/peer/adapter/outbound/commandrunner"
	"github.com/bentos-lab/peer/domain"
	"github.com/bentos-lab/peer/usecase"
	"github.com/bentos-lab/peer/usecase/contracts"
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
	UseCwd       bool
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
	isLocalCopy  bool
	cleanup      bool
	useCwd       bool
	mu           sync.Mutex
}

const hostCodeEnvironmentTempBaseDirName = ".peertmp"
const hostCodeEnvironmentFetchedRefPrefix = "refs/peer/fetched/"

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
		cleanup:      cfg.IsRemote,
		useCwd:       cfg.UseCwd,
	}
}

// SetupAgent checks out the requested head ref and returns a host-backed coding agent.
func (e *HostCodeEnvironment) SetupAgent(ctx context.Context, opts domain.CodingAgentSetupOptions) (contracts.CodingAgent, error) {
	agentName := strings.ToLower(strings.TrimSpace(opts.Agent))
	if agentName == "" {
		return nil, fmt.Errorf("agent is required")
	}

	headRef := strings.TrimSpace(opts.Ref)
	workspaceDir, err := e.workspaceDirForRef(headRef)
	if err != nil {
		return nil, err
	}
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

// ResolveBaseHead resolves base/head refs into concrete commit refs when needed.
func (e *HostCodeEnvironment) ResolveBaseHead(ctx context.Context, base string, head string) (string, string, error) {
	base = strings.TrimSpace(base)
	head = strings.TrimSpace(head)
	if base == "" {
		return "", "", fmt.Errorf("base ref is required")
	}
	if head == "" {
		return "", "", fmt.Errorf("head ref is required")
	}
	if strings.HasPrefix(base, "@") {
		return "", "", fmt.Errorf("base ref must not use workspace tokens")
	}
	if strings.HasPrefix(head, "@") && head != "@staged" && head != "@all" {
		return "", "", fmt.Errorf("head ref must not use workspace tokens")
	}

	workspaceDir, err := e.workspaceDirForRef(head)
	if err != nil {
		return "", "", err
	}
	if e.isRemote && isWorkspaceTokenRef(head) {
		return "", "", fmt.Errorf("ref %q requires local workspace mode", head)
	}

	if isWorkspaceTokenRef(head) {
		preCommitHead, err := e.getCurrentHead(ctx, workspaceDir)
		if err != nil {
			return "", "", err
		}

		stageAll := head == "@all"
		if _, err := e.CommitChanges(ctx, domain.CodeEnvironmentCommitOptions{
			CommitMessage: "peer: snapshot workspace",
			StageAll:      stageAll,
		}); err != nil {
			return "", "", err
		}

		newHead, err := e.getCurrentHead(ctx, workspaceDir)
		if err != nil {
			return "", "", err
		}

		resolvedBase := base
		if base == "HEAD" {
			resolvedBase = preCommitHead
		}
		resolvedBase = strings.TrimSpace(resolvedBase)
		if resolvedBase == "" || strings.HasPrefix(resolvedBase, "@") {
			return "", "", fmt.Errorf("base ref must not use workspace tokens")
		}
		return resolvedBase, newHead, nil
	}

	resolvedBase, err := e.normalizeRef(ctx, workspaceDir, base)
	if err != nil {
		return "", "", err
	}
	resolvedHead, err := e.normalizeRef(ctx, workspaceDir, head)
	if err != nil {
		return "", "", err
	}
	return resolvedBase, resolvedHead, nil
}

// LoadChangedFiles resolves changed files from the selected comparison mode.
func (e *HostCodeEnvironment) LoadChangedFiles(ctx context.Context, opts domain.CodeEnvironmentLoadOptions) ([]domain.ChangedFile, error) {
	workspaceDir, err := e.workspaceDirForRef(opts.Head)
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
	workspaceDir, err := e.workspaceDirForRef(ref)
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
			e.logger.Warnf("ReadFile failed for path %q at ref %q: %v", path, ref, err)
			return "", false, nil
		}
		return content, found, nil
	}

	normalizedRef, err := e.normalizeRef(ctx, workspaceDir, ref)
	if err != nil {
		return "", false, err
	}

	content, found, err := e.readRefFile(ctx, workspaceDir, path, normalizedRef)
	if err != nil {
		e.logger.Warnf("ReadFile failed for path %q at ref %q: %v", path, normalizedRef, err)
		return "", false, nil
	}
	return content, found, nil
}

// Cleanup removes any temporary workspace created for remote repositories.
func (e *HostCodeEnvironment) Cleanup(_ context.Context) error {
	if !e.cleanup {
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

// CommitChanges commits workspace changes.
func (e *HostCodeEnvironment) CommitChanges(ctx context.Context, opts domain.CodeEnvironmentCommitOptions) (domain.CodeEnvironmentCommitResult, error) {
	workspaceDir, err := e.workspaceDirForRun()
	if err != nil {
		return domain.CodeEnvironmentCommitResult{}, err
	}

	commitMessage := strings.TrimSpace(opts.CommitMessage)
	if commitMessage == "" {
		return domain.CodeEnvironmentCommitResult{}, fmt.Errorf("commit message is required")
	}

	if opts.StageAll {
		if _, err := e.git(ctx, workspaceDir, "add", "-A"); err != nil {
			return domain.CodeEnvironmentCommitResult{}, err
		}
	}

	staged, err := e.git(ctx, workspaceDir, "diff", "--cached", "--name-only")
	if err != nil {
		return domain.CodeEnvironmentCommitResult{}, err
	}
	if strings.TrimSpace(string(staged.Stdout)) == "" {
		return domain.CodeEnvironmentCommitResult{}, domain.ErrNoCodeChanges
	}

	if _, err := e.git(ctx, workspaceDir, "commit", "-m", commitMessage); err != nil {
		return domain.CodeEnvironmentCommitResult{}, err
	}
	return domain.CodeEnvironmentCommitResult{Committed: true}, nil
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
