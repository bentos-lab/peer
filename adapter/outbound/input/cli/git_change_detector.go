package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"bentos-backend/adapter/outbound/commandrunner"
)

// GitChangeDetector discovers changed files from local git state.
type GitChangeDetector struct {
	runner commandrunner.Runner
}

// NewGitChangeDetector creates a git-backed change detector.
func NewGitChangeDetector() *GitChangeDetector {
	return newGitChangeDetectorWithRunner(commandrunner.NewOSCommandRunner())
}

func newGitChangeDetectorWithRunner(runner commandrunner.Runner) *GitChangeDetector {
	return &GitChangeDetector{runner: runner}
}

// ListStaged returns staged changed file paths.
func (d *GitChangeDetector) ListStaged(ctx context.Context) ([]string, error) {
	return d.list(ctx, "diff", "--cached", "--name-only", "--diff-filter=ACMRTUXB")
}

// ListUnstaged returns unstaged changed file paths.
func (d *GitChangeDetector) ListUnstaged(ctx context.Context) ([]string, error) {
	return d.list(ctx, "diff", "--name-only", "--diff-filter=ACMRTUXB")
}

// ListUntracked returns untracked file paths.
func (d *GitChangeDetector) ListUntracked(ctx context.Context) ([]string, error) {
	return d.list(ctx, "ls-files", "--others", "--exclude-standard")
}

// GetDiffForPath returns unified diff content for one path.
func (d *GitChangeDetector) GetDiffForPath(ctx context.Context, path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", nil
	}

	stagedDiff, err := d.raw(ctx, "diff", "--cached", "--", path)
	if err != nil {
		return "", err
	}
	unstagedDiff, err := d.raw(ctx, "diff", "--", path)
	if err != nil {
		return "", err
	}

	stagedDiff = strings.TrimSpace(stagedDiff)
	unstagedDiff = strings.TrimSpace(unstagedDiff)
	switch {
	case stagedDiff != "" && unstagedDiff != "":
		return stagedDiff + "\n\n" + unstagedDiff, nil
	case stagedDiff != "":
		return stagedDiff, nil
	default:
		return unstagedDiff, nil
	}
}

func (d *GitChangeDetector) list(ctx context.Context, args ...string) ([]string, error) {
	if d.runner == nil {
		return nil, errors.New("git runner is required")
	}

	stdout, err := d.raw(ctx, args...)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(stdout), "\n")
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

func (d *GitChangeDetector) raw(ctx context.Context, args ...string) (string, error) {
	if d.runner == nil {
		return "", errors.New("git runner is required")
	}

	result, err := d.runner.Run(ctx, "git", args...)
	if err != nil {
		output := strings.TrimSpace(string(result.Stderr))
		if output == "" {
			output = strings.TrimSpace(string(result.Stdout))
		}
		if output == "" {
			return "", fmt.Errorf("git %s failed: %w", strings.Join(args, " "), err)
		}
		return "", fmt.Errorf("git %s failed: %w: %s", strings.Join(args, " "), err, output)
	}

	return string(result.Stdout), nil
}
