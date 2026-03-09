package host

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"bentos-backend/adapter/outbound/commandrunner"
	"bentos-backend/domain"
	"bentos-backend/shared/logger/stdlogger"
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
}

const hostCodeEnvironmentTempBaseDirName = ".sisutmp"
const hostCodeEnvironmentFetchedRefPrefix = "refs/bentos/fetched/"

// NewHostCodeEnvironment creates a host-backed code environment.
func NewHostCodeEnvironment() *HostCodeEnvironment {
	return NewHostCodeEnvironmentWithConfig(HostCodeEnvironmentConfig{
		Runner:      commandrunner.NewOSCommandRunner(),
		AgentRunner: commandrunner.NewOSStreamCommandRunner(),
		Getwd:       os.Getwd,
		MakeTempDir: newHostCodeEnvironmentTempDirMaker(os.UserHomeDir),
		Logger:      stdlogger.Nop(),
	})
}

// NewHostCodeEnvironmentWithLogger creates a host-backed code environment with the provided logger.
func NewHostCodeEnvironmentWithLogger(logger usecase.Logger) *HostCodeEnvironment {
	return NewHostCodeEnvironmentWithConfig(HostCodeEnvironmentConfig{
		Runner:      commandrunner.NewOSCommandRunner(),
		AgentRunner: commandrunner.NewOSStreamCommandRunner(),
		Getwd:       os.Getwd,
		MakeTempDir: newHostCodeEnvironmentTempDirMaker(os.UserHomeDir),
		Logger:      logger,
	})
}

// NewHostCodeEnvironmentWithConfig creates a host environment with injected dependencies.
func NewHostCodeEnvironmentWithConfig(cfg HostCodeEnvironmentConfig) *HostCodeEnvironment {
	runner := cfg.Runner
	if runner == nil {
		runner = commandrunner.NewOSCommandRunner()
	}
	agentRunner := cfg.AgentRunner
	if agentRunner == nil {
		agentRunner = commandrunner.NewOSStreamCommandRunner()
	}
	getwd := cfg.Getwd
	if getwd == nil {
		getwd = os.Getwd
	}
	makeTempDir := cfg.MakeTempDir
	if makeTempDir == nil {
		makeTempDir = newHostCodeEnvironmentTempDirMaker(os.UserHomeDir)
	}
	logger := cfg.Logger
	if logger == nil {
		logger = stdlogger.Nop()
	}
	return &HostCodeEnvironment{
		runner:       runner,
		agentRunner:  agentRunner,
		getwd:        getwd,
		makeTempDir:  makeTempDir,
		logger:       logger,
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

	base := strings.TrimSpace(opts.Base)
	head := strings.TrimSpace(opts.Head)
	resolvedBase := base
	resolvedHead := head
	if !isWorkspaceTokenRef(head) {
		if resolvedBase == "" {
			resolvedBase = "HEAD"
		}
		var resolveErr error
		resolvedBase, resolveErr = e.resolveRef(ctx, workspaceDir, resolvedBase)
		if resolveErr != nil {
			return nil, resolveErr
		}
		resolvedHead, resolveErr = e.resolveRef(ctx, workspaceDir, resolvedHead)
		if resolveErr != nil {
			return nil, resolveErr
		}
	}

	var paths []string
	switch head {
	case "", "@staged":
		paths, err = e.listChangedPaths(ctx, workspaceDir, "diff", "--cached", "--name-only", "--diff-filter=ACMRTUXB")
	case "@all":
		staged, listErr := e.listChangedPaths(ctx, workspaceDir, "diff", "--cached", "--name-only", "--diff-filter=ACMRTUXB")
		if listErr != nil {
			return nil, listErr
		}
		unstaged, listErr := e.listChangedPaths(ctx, workspaceDir, "diff", "--name-only", "--diff-filter=ACMRTUXB")
		if listErr != nil {
			return nil, listErr
		}
		untracked, listErr := e.listChangedPaths(ctx, workspaceDir, "ls-files", "--others", "--exclude-standard")
		if listErr != nil {
			return nil, listErr
		}
		paths = dedupePaths(append(append(staged, unstaged...), untracked...))
	default:
		paths, err = e.listChangedPaths(ctx, workspaceDir, "diff", "--name-only", "--diff-filter=ACMRTUXB", fmt.Sprintf("%s..%s", resolvedBase, resolvedHead))
	}
	if err != nil {
		return nil, err
	}

	files := make([]domain.ChangedFile, 0, len(paths))
	for _, path := range paths {
		content, readErr := e.readPathContent(ctx, workspaceDir, path, resolvedBase, resolvedHead)
		if readErr != nil {
			return nil, readErr
		}

		diffSnippet, diffErr := e.diffForPath(ctx, workspaceDir, path, resolvedBase, resolvedHead)
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
		return nil, fmt.Errorf("no changes found for base %q and head %q", base, head)
	}
	return files, nil
}

func (e *HostCodeEnvironment) verifyLocalRefExists(ctx context.Context, workspaceDir string, ref string) error {
	result, err := e.runner.Run(ctx, "git", "-C", workspaceDir, "rev-parse", "--verify", fmt.Sprintf("%s^{commit}", ref))
	if err != nil {
		return fmt.Errorf("failed to verify ref %q: %w", ref, formatCommandError(err, result))
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

	e.workspaceDir = workspaceDir
	e.isRemote = true
	return nil
}

func (e *HostCodeEnvironment) workspaceDirForRun() (string, error) {
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

	resolvedHeadRef, err := e.resolveRef(ctx, workspaceDir, headRef)
	if err != nil {
		return err
	}

	result, err := e.runner.Run(ctx, "git", "-C", workspaceDir, "checkout", resolvedHeadRef)
	if err != nil {
		return fmt.Errorf("failed to checkout ref %q: %w", resolvedHeadRef, formatCommandError(err, result))
	}

	return nil
}

func (e *HostCodeEnvironment) resolveRef(ctx context.Context, workspaceDir string, requestedRef string) (string, error) {
	requestedRef = strings.TrimSpace(requestedRef)
	if requestedRef == "" {
		return "", fmt.Errorf("ref is required")
	}
	if err := e.verifyLocalRefExists(ctx, workspaceDir, requestedRef); err == nil {
		return requestedRef, nil
	}

	fetchedRef := localFetchedRefName(requestedRef)
	if err := e.verifyLocalRefExists(ctx, workspaceDir, fetchedRef); err == nil {
		return fetchedRef, nil
	}

	candidates := refFetchCandidates(requestedRef)
	var lastErr error
	for _, candidate := range candidates {
		result, fetchErr := e.runner.Run(ctx, "git", "-C", workspaceDir, "fetch", "origin", fmt.Sprintf("%s:%s", candidate, fetchedRef))
		if fetchErr != nil {
			lastErr = formatCommandError(fetchErr, result)
			continue
		}
		if err := e.verifyLocalRefExists(ctx, workspaceDir, fetchedRef); err == nil {
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
	result, err := e.runner.Run(ctx, "git", append([]string{"-C", workspaceDir}, args...)...)
	if err != nil {
		return nil, fmt.Errorf("failed to list changed paths: %w", formatCommandError(err, result))
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

func (e *HostCodeEnvironment) readPathContent(ctx context.Context, workspaceDir string, path string, _, _ string) (string, error) {
	_ = ctx
	raw, err := os.ReadFile(filepath.Join(workspaceDir, path))
	if err != nil {
		return "", fmt.Errorf("failed to read file content for %q: %w", path, err)
	}
	return string(raw), nil
}

func (e *HostCodeEnvironment) diffForPath(ctx context.Context, workspaceDir string, path string, base string, head string) (string, error) {
	var args []string
	switch strings.TrimSpace(head) {
	case "", "@staged":
		args = []string{"-C", workspaceDir, "diff", "--cached", "--", path}
	case "@all":
		stagedResult, err := e.runner.Run(ctx, "git", "-C", workspaceDir, "diff", "--cached", "--", path)
		if err != nil {
			return "", fmt.Errorf("failed to get staged diff for %q: %w", path, formatCommandError(err, stagedResult))
		}
		unstagedResult, err := e.runner.Run(ctx, "git", "-C", workspaceDir, "diff", "--", path)
		if err != nil {
			return "", fmt.Errorf("failed to get unstaged diff for %q: %w", path, formatCommandError(err, unstagedResult))
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
		args = []string{"-C", workspaceDir, "diff", fmt.Sprintf("%s..%s", base, head), "--", path}
	}
	result, err := e.runner.Run(ctx, "git", args...)
	if err != nil {
		return "", fmt.Errorf("failed to get diff for %q: %w", path, formatCommandError(err, result))
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
