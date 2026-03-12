package host

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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

func (e *HostCodeEnvironment) verifyLocalRefExists(ctx context.Context, workspaceDir string, ref string) error {
	_, err := e.git(ctx, workspaceDir, "rev-parse", "--verify", fmt.Sprintf("%s^{commit}", ref))
	if err != nil {
		return fmt.Errorf("failed to verify ref %q: %w", ref, err)
	}
	return nil
}

func (e *HostCodeEnvironment) prepareWorkspace(ctx context.Context, repoURL string) error {
	repoURL = strings.TrimSpace(repoURL)
	if repoURL == "" {
		workspaceDir, err := e.getwd()
		if err != nil {
			return fmt.Errorf("failed to resolve current workspace directory: %w", err)
		}
		e.workspaceDir = workspaceDir
		e.isRemote = false
		return nil
	}

	workspaceDir, err := e.makeTempDir()
	if err != nil {
		return fmt.Errorf("failed to create temporary workspace directory: %w", err)
	}
	e.logger.Debugf("Code environment temporary workspace directory is %q under tmp folder %q.", workspaceDir, filepath.Dir(workspaceDir))

	result, err := e.runner.Run(ctx, "git", "clone", "--depth", "1", repoURL, workspaceDir)
	if err != nil {
		return fmt.Errorf("failed to clone repository: %w", formatCommandError(err, result))
	}

	e.logger.Debugf("Cloned repo to %s (shallow=true)", workspaceDir)
	e.workspaceDir = workspaceDir
	e.isRemote = true
	return nil
}

func (e *HostCodeEnvironment) workspaceDirForRun() (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if strings.TrimSpace(e.workspaceDir) == "" {
		workspaceDir, err := e.getwd()
		if err != nil {
			return "", fmt.Errorf("failed to resolve current workspace directory: %w", err)
		}
		e.workspaceDir = workspaceDir
		e.isRemote = false
	}
	return e.workspaceDir, nil
}

func (e *HostCodeEnvironment) syncRef(ctx context.Context, workspaceDir string, headRef string) error {
	if isWorkspaceTokenRef(headRef) {
		return nil
	}

	currentHead, headErr := e.getCurrentHead(ctx, workspaceDir)
	if headErr != nil {
		e.logger.Debugf("Failed to get current HEAD before sync: %v", headErr)
	} else {
		e.logger.Debugf("Current HEAD before sync: %s", currentHead)
	}

	resolvedHeadRef, err := e.resolveRef(ctx, workspaceDir, headRef)
	if err != nil {
		return err
	}

	e.logger.Debugf("Syncing ref: requested=%s, resolved=%s", headRef, resolvedHeadRef)
	_, err = e.git(ctx, workspaceDir, "checkout", resolvedHeadRef)
	if err != nil {
		return fmt.Errorf("failed to checkout ref %q: %w", resolvedHeadRef, err)
	}

	currentHead, headErr = e.getCurrentHead(ctx, workspaceDir)
	if headErr != nil {
		e.logger.Debugf("Failed to get current HEAD after sync: %v", headErr)
	} else {
		e.logger.Debugf("Current HEAD after sync: %s", currentHead)
	}

	return nil
}

func (e *HostCodeEnvironment) resolveRef(ctx context.Context, workspaceDir string, requestedRef string) (string, error) {
	requestedRef = strings.TrimSpace(requestedRef)
	if requestedRef == "" {
		return "", fmt.Errorf("ref is required")
	}
	e.logger.Debugf("Resolving ref: %s (isRemote=%v)", requestedRef, e.isRemote)
	if err := e.verifyLocalRefExists(ctx, workspaceDir, requestedRef); err == nil {
		e.logger.Debugf("Ref found locally: requested=%s", requestedRef)
		return requestedRef, nil
	}

	fetchedRef := localFetchedRefName(requestedRef)
	if err := e.verifyLocalRefExists(ctx, workspaceDir, fetchedRef); err == nil {
		e.logger.Debugf("Ref found in fetched cache: requested=%s, resolved=%s", requestedRef, fetchedRef)
		return fetchedRef, nil
	}

	candidates := refFetchCandidates(requestedRef)
	var lastErr error
	isShallow, shallowErr := e.isShallowRepository(ctx, workspaceDir)
	if shallowErr != nil {
		e.logger.Debugf("Failed to determine if repository is shallow: %v", shallowErr)
	}
	for _, candidate := range candidates {
		e.logger.Debugf("Attempting to fetch ref candidate: %s, will store as: %s", candidate, fetchedRef)
		args := []string{"-C", workspaceDir, "fetch"}
		if shallowErr == nil && isShallow {
			args = append(args, "--unshallow")
		}
		args = append(args, "origin", fmt.Sprintf("%s:%s", candidate, fetchedRef))
		result, fetchErr := e.runner.Run(ctx, "git", args...)
		if fetchErr != nil {
			lastErr = formatCommandError(fetchErr, result)
			continue
		}
		if err := e.verifyLocalRefExists(ctx, workspaceDir, fetchedRef); err == nil {
			e.logger.Debugf("Successfully fetched ref: candidate=%s -> %s", candidate, fetchedRef)
			return fetchedRef, nil
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("unknown fetch error")
	}
	return "", fmt.Errorf("failed to resolve ref %q in local workspace: %w", requestedRef, lastErr)
}

func refFetchCandidates(requestedRef string) []string {
	requestedRef = strings.TrimSpace(requestedRef)
	candidates := []string{requestedRef}
	if !strings.HasPrefix(requestedRef, "refs/") {
		candidates = append(candidates, "refs/heads/"+requestedRef, "refs/tags/"+requestedRef)
	}
	return dedupePaths(candidates)
}

func localFetchedRefName(requestedRef string) string {
	sum := sha1.Sum([]byte(strings.TrimSpace(requestedRef)))
	return hostCodeEnvironmentFetchedRefPrefix + hex.EncodeToString(sum[:])
}

func isWorkspaceTokenRef(ref string) bool {
	switch strings.TrimSpace(ref) {
	case "", "@staged", "@all":
		return true
	default:
		return false
	}
}

func (e *HostCodeEnvironment) listChangedPaths(ctx context.Context, workspaceDir string, args ...string) ([]string, error) {
	result, err := e.git(ctx, workspaceDir, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list changed paths: %w", err)
	}
	lines := strings.Split(strings.TrimSpace(string(result.Stdout)), "\n")
	paths := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		paths = append(paths, line)
	}
	return paths, nil
}

func (e *HostCodeEnvironment) readPathContent(ctx context.Context, workspaceDir string, path string, head string) (string, error) {
	if isWorkspaceTokenRef(head) {
		raw, err := os.ReadFile(filepath.Join(workspaceDir, path))
		if err == nil {
			return string(raw), nil
		}
		e.logger.Debugf("Failed to read file %s from workspace: %v", path, err)

		// For staged content that is missing from the working tree, read from index.
		result, showErr := e.git(ctx, workspaceDir, "show", fmt.Sprintf(":%s", path))
		if showErr != nil {
			return "", fmt.Errorf("failed to read file content for %q: %w (index fallback failed: %v)", path, err, showErr)
		}
		return strings.TrimSpace(string(result.Stdout)), nil
	}
	result, err := e.git(ctx, workspaceDir, "show", fmt.Sprintf("%s:%s", head, path))
	if err != nil {
		return "", fmt.Errorf("failed to read file content for %q at ref %q: %w", path, head, err)
	}
	return strings.TrimSpace(string(result.Stdout)), nil
}

func (e *HostCodeEnvironment) readWorkspaceFile(ctx context.Context, workspaceDir string, path string) (string, bool, error) {
	raw, err := os.ReadFile(filepath.Join(workspaceDir, path))
	if err == nil {
		return string(raw), true, nil
	}
	readErr := err
	if !errors.Is(readErr, os.ErrNotExist) {
		e.logger.Debugf("Failed to read file %s from workspace: %v", path, readErr)
	}

	result, showErr := e.git(ctx, workspaceDir, "show", fmt.Sprintf(":%s", path))
	if showErr != nil {
		if isGitPathMissing(showErr) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("failed to read file content for %q: %w (index fallback failed: %v)", path, readErr, showErr)
	}
	return strings.TrimSpace(string(result.Stdout)), true, nil
}

func (e *HostCodeEnvironment) readRefFile(ctx context.Context, workspaceDir string, path string, ref string) (string, bool, error) {
	_, err := e.git(ctx, workspaceDir, "cat-file", "-e", fmt.Sprintf("%s:%s", ref, path))
	if err != nil {
		if isGitPathMissing(err) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("failed to verify file %q at ref %q: %w", path, ref, err)
	}

	result, err := e.git(ctx, workspaceDir, "show", fmt.Sprintf("%s:%s", ref, path))
	if err != nil {
		if isGitPathMissing(err) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("failed to read file content for %q at ref %q: %w", path, ref, err)
	}
	return strings.TrimSpace(string(result.Stdout)), true, nil
}

func (e *HostCodeEnvironment) getCurrentHead(ctx context.Context, workspaceDir string) (string, error) {
	result, err := e.git(ctx, workspaceDir, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(result.Stdout)), nil
}

func (e *HostCodeEnvironment) mergeBase(ctx context.Context, workspaceDir string, base string, head string) (string, error) {
	result, err := e.git(ctx, workspaceDir, "merge-base", base, head)
	if err != nil {
		return "", fmt.Errorf("failed to resolve merge-base for %q and %q: %w", base, head, err)
	}
	mergeBase := strings.TrimSpace(string(result.Stdout))
	if mergeBase == "" {
		return "", fmt.Errorf("failed to resolve merge-base for %q and %q: empty result", base, head)
	}
	return mergeBase, nil
}

func (e *HostCodeEnvironment) diffForPath(ctx context.Context, workspaceDir string, path string, base string, head string) (string, error) {
	var args []string
	switch strings.TrimSpace(head) {
	case "", "@staged":
		args = []string{"diff", "--cached", "--", path}
	case "@all":
		stagedResult, err := e.git(ctx, workspaceDir, "diff", "--cached", "--", path)
		if err != nil {
			return "", fmt.Errorf("failed to get staged diff for %q: %w", path, err)
		}
		unstagedResult, err := e.git(ctx, workspaceDir, "diff", "--", path)
		if err != nil {
			return "", fmt.Errorf("failed to get unstaged diff for %q: %w", path, err)
		}
		staged := strings.TrimSpace(string(stagedResult.Stdout))
		unstaged := strings.TrimSpace(string(unstagedResult.Stdout))
		if staged != "" && unstaged != "" {
			return staged + "\n\n" + unstaged, nil
		}
		if staged != "" {
			return staged, nil
		}
		return unstaged, nil
	default:
		if strings.TrimSpace(base) == "" {
			base = "HEAD"
		}
		args = []string{"diff", fmt.Sprintf("%s..%s", base, head), "--", path}
	}
	result, err := e.git(ctx, workspaceDir, args...)
	if err != nil {
		return "", fmt.Errorf("failed to get diff for %q: %w", path, err)
	}
	return strings.TrimSpace(string(result.Stdout)), nil
}

func dedupePaths(paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	result := make([]string, 0, len(paths))
	for _, path := range paths {
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		result = append(result, path)
	}
	return result
}

func formatCommandError(err error, result commandrunner.Result) error {
	stderr := strings.TrimSpace(string(result.Stderr))
	if stderr != "" {
		return fmt.Errorf("%w: %s", err, stderr)
	}

	stdout := strings.TrimSpace(string(result.Stdout))
	if stdout != "" {
		return fmt.Errorf("%w: %s", err, stdout)
	}

	return err
}

func isGitPathMissing(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "does not exist") {
		return true
	}
	if strings.Contains(message, "not in the index") {
		return true
	}
	if strings.Contains(message, "unknown revision or path not in the working tree") {
		return true
	}
	if strings.Contains(message, "ambiguous argument") {
		return true
	}
	if strings.Contains(message, "pathspec") && strings.Contains(message, "did not match") {
		return true
	}
	return false
}

func (e *HostCodeEnvironment) resolveDiffRefs(ctx context.Context, workspaceDir string, base string, head string) (string, string, string, error) {
	base = strings.TrimSpace(base)
	head = strings.TrimSpace(head)
	resolvedBase := base
	resolvedHead := head
	mergeBase := resolvedBase
	if isWorkspaceTokenRef(head) {
		return resolvedBase, resolvedHead, mergeBase, nil
	}

	if resolvedBase == "" {
		resolvedBase = "HEAD"
	}
	var resolveErr error
	resolvedBase, resolveErr = e.resolveRef(ctx, workspaceDir, resolvedBase)
	if resolveErr != nil {
		return "", "", "", resolveErr
	}
	resolvedHead, resolveErr = e.resolveRef(ctx, workspaceDir, resolvedHead)
	if resolveErr != nil {
		return "", "", "", resolveErr
	}

	mergeBase, resolveErr = e.mergeBase(ctx, workspaceDir, resolvedBase, resolvedHead)
	if resolveErr != nil {
		return "", "", "", resolveErr
	}

	return resolvedBase, resolvedHead, mergeBase, nil
}

func (e *HostCodeEnvironment) collectChangedPaths(ctx context.Context, workspaceDir string, head string, mergeBase string, resolvedHead string) ([]string, error) {
	switch head {
	case "", "@staged":
		return e.listChangedPaths(ctx, workspaceDir, "diff", "--cached", "--name-only", "--diff-filter=ACMRTUXB")
	case "@all":
		staged, err := e.listChangedPaths(ctx, workspaceDir, "diff", "--cached", "--name-only", "--diff-filter=ACMRTUXB")
		if err != nil {
			return nil, err
		}
		unstaged, err := e.listChangedPaths(ctx, workspaceDir, "diff", "--name-only", "--diff-filter=ACMRTUXB")
		if err != nil {
			return nil, err
		}
		untracked, err := e.listChangedPaths(ctx, workspaceDir, "ls-files", "--others", "--exclude-standard")
		if err != nil {
			return nil, err
		}
		return dedupePaths(append(append(staged, unstaged...), untracked...)), nil
	default:
		return e.listChangedPaths(ctx, workspaceDir, "diff", "--name-only", "--diff-filter=ACMRTUXB", fmt.Sprintf("%s..%s", mergeBase, resolvedHead))
	}
}

func (e *HostCodeEnvironment) git(ctx context.Context, workspaceDir string, args ...string) (commandrunner.Result, error) {
	result, err := e.runner.Run(ctx, "git", append([]string{"-C", workspaceDir}, args...)...)
	if err != nil {
		return result, formatCommandError(err, result)
	}
	return result, nil
}

func (e *HostCodeEnvironment) isShallowRepository(ctx context.Context, workspaceDir string) (bool, error) {
	result, err := e.git(ctx, workspaceDir, "rev-parse", "--is-shallow-repository")
	if err != nil {
		return false, err
	}
	value := strings.TrimSpace(string(result.Stdout))
	switch value {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf("unexpected shallow repository value %q", value)
	}
}

func newHostCodeEnvironmentTempDirMaker(userHomeDir func() (string, error)) func() (string, error) {
	return func() (string, error) {
		homeDir, err := userHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to resolve user home directory: %w", err)
		}
		homeDir = strings.TrimSpace(homeDir)
		if homeDir == "" {
			return "", fmt.Errorf("failed to resolve user home directory: empty path")
		}

		baseDir := filepath.Join(homeDir, hostCodeEnvironmentTempBaseDirName)
		if err := os.MkdirAll(baseDir, 0o700); err != nil {
			return "", fmt.Errorf("failed to create temporary workspace base directory %q: %w", baseDir, err)
		}
		if err := os.Chmod(baseDir, 0o700); err != nil {
			return "", fmt.Errorf("failed to secure temporary workspace base directory %q: %w", baseDir, err)
		}

		workspaceDir, err := os.MkdirTemp(baseDir, "bentos-coding-agent-*")
		if err != nil {
			return "", fmt.Errorf("failed to create temporary workspace directory under %q: %w", baseDir, err)
		}
		return workspaceDir, nil
	}
}
