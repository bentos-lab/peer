package cli

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// GitChangeDetector discovers changed files from local git state.
type GitChangeDetector struct {
	runGit func(ctx context.Context, args ...string) ([]byte, error)
}

// NewGitChangeDetector creates a git-backed change detector.
func NewGitChangeDetector() *GitChangeDetector {
	return newGitChangeDetectorWithRunner(func(ctx context.Context, args ...string) ([]byte, error) {
		cmd := exec.CommandContext(ctx, "git", args...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("git %s failed: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
		}
		return out, nil
	})
}

func newGitChangeDetectorWithRunner(runGit func(ctx context.Context, args ...string) ([]byte, error)) *GitChangeDetector {
	return &GitChangeDetector{runGit: runGit}
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
	if d.runGit == nil {
		return nil, errors.New("git runner is required")
	}

	out, err := d.runGit(ctx, args...)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
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
