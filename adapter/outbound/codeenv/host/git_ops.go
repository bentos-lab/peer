package host

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bentos-lab/peer/adapter/outbound/commandrunner"
)

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
	resolvedBase, resolveErr = e.normalizeRef(ctx, workspaceDir, resolvedBase)
	if resolveErr != nil {
		return "", "", "", resolveErr
	}
	resolvedHead, resolveErr = e.normalizeRef(ctx, workspaceDir, resolvedHead)
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
