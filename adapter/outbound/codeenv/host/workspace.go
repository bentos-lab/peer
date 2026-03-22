package host

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

func (e *HostCodeEnvironment) prepareWorkspace(ctx context.Context, repoURL string) error {
	repoURL = strings.TrimSpace(repoURL)
	if e.useCwd {
		if repoURL != "" {
			return fmt.Errorf("cwd mode only allowed for local repository")
		}
		workspaceDir, err := e.getwd()
		if err != nil {
			return fmt.Errorf("failed to resolve current workspace directory: %w", err)
		}
		workspaceDir = strings.TrimSpace(workspaceDir)
		if workspaceDir == "" {
			return fmt.Errorf("failed to resolve current workspace directory: empty path")
		}
		e.workspaceDir = workspaceDir
		e.isRemote = false
		e.isLocalCopy = false
		e.cleanup = false
		return nil
	}
	if repoURL == "" {
		_, err := e.ensureLocalCopyWorkspace()
		return err
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
	e.isLocalCopy = false
	e.cleanup = true
	return nil
}

func (e *HostCodeEnvironment) workspaceDirForRun() (string, error) {
	return e.ensureLocalCopyWorkspace()
}

func (e *HostCodeEnvironment) workspaceDirForRef(ref string) (string, error) {
	ref = strings.TrimSpace(ref)

	if isWorkspaceTokenRef(ref) {
		return e.workspaceDirForRun()
	}

	e.mu.Lock()
	if strings.TrimSpace(e.workspaceDir) != "" {
		workspaceDir := e.workspaceDir
		e.mu.Unlock()
		return workspaceDir, nil
	}
	e.mu.Unlock()

	return e.ensureLocalCopyWorkspace()
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

	resolvedHeadRef, err := e.normalizeRef(ctx, workspaceDir, headRef)
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

func (e *HostCodeEnvironment) ensureLocalCopyWorkspace() (string, error) {
	e.mu.Lock()
	if strings.TrimSpace(e.workspaceDir) != "" {
		workspaceDir := e.workspaceDir
		e.mu.Unlock()
		return workspaceDir, nil
	}
	e.mu.Unlock()

	sourceDir, err := e.getwd()
	if err != nil {
		return "", fmt.Errorf("failed to resolve current workspace directory: %w", err)
	}
	sourceDir = strings.TrimSpace(sourceDir)
	if sourceDir == "" {
		return "", fmt.Errorf("failed to resolve current workspace directory: empty path")
	}
	if e.useCwd {
		e.mu.Lock()
		e.workspaceDir = sourceDir
		e.isRemote = false
		e.isLocalCopy = false
		e.cleanup = false
		e.mu.Unlock()
		return sourceDir, nil
	}

	workspaceDir, err := e.makeTempDir()
	if err != nil {
		return "", fmt.Errorf("failed to create temporary workspace directory: %w", err)
	}
	e.logger.Debugf("Code environment temporary workspace directory is %q under tmp folder %q.", workspaceDir, filepath.Dir(workspaceDir))

	if err := copyWorkspaceDir(sourceDir, workspaceDir); err != nil {
		return "", fmt.Errorf("failed to copy workspace to %q: %w", workspaceDir, err)
	}
	e.logger.Debugf("Copied workspace to %s", workspaceDir)

	e.mu.Lock()
	e.workspaceDir = workspaceDir
	e.isRemote = false
	e.isLocalCopy = true
	e.cleanup = true
	e.mu.Unlock()
	return workspaceDir, nil
}
