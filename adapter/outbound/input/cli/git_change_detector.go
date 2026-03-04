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

func (d *GitChangeDetector) list(ctx context.Context, args ...string) ([]string, error) {
	if d.runner == nil {
		return nil, errors.New("git runner is required")
	}

	result, err := d.runner.Run(ctx, "git", args...)
	if err != nil {
		output := strings.TrimSpace(string(result.Stderr))
		if output == "" {
			output = strings.TrimSpace(string(result.Stdout))
		}
		if output == "" {
			return nil, fmt.Errorf("git %s failed: %w", strings.Join(args, " "), err)
		}
		return nil, fmt.Errorf("git %s failed: %w: %s", strings.Join(args, " "), err, output)
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
