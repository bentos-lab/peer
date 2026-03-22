package host

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func copyWorkspaceDir(sourceDir string, workspaceDir string) error {
	sourceInfo, err := os.Stat(sourceDir)
	if err != nil {
		return fmt.Errorf("failed to stat source workspace %q: %w", sourceDir, err)
	}
	if !sourceInfo.IsDir() {
		return fmt.Errorf("source workspace %q is not a directory", sourceDir)
	}

	return filepath.WalkDir(sourceDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return fmt.Errorf("failed to resolve workspace path for %q: %w", path, err)
		}
		if rel == "." {
			return nil
		}
		targetPath := filepath.Join(workspaceDir, rel)

		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("failed to stat workspace path %q: %w", path, err)
		}
		mode := info.Mode()
		switch {
		case mode&os.ModeSymlink != 0:
			link, err := os.Readlink(path)
			if err != nil {
				return fmt.Errorf("failed to read symlink %q: %w", path, err)
			}
			if err := os.Symlink(link, targetPath); err != nil {
				return fmt.Errorf("failed to create symlink %q: %w", targetPath, err)
			}
			return nil
		case mode.IsDir():
			if err := os.MkdirAll(targetPath, mode.Perm()); err != nil {
				return fmt.Errorf("failed to create directory %q: %w", targetPath, err)
			}
			if err := os.Chmod(targetPath, mode.Perm()); err != nil {
				return fmt.Errorf("failed to set permissions on directory %q: %w", targetPath, err)
			}
			return nil
		case mode.IsRegular():
			if err := copyWorkspaceFile(path, targetPath, mode.Perm()); err != nil {
				return err
			}
			return nil
		default:
			return fmt.Errorf("unsupported file type %q", path)
		}
	})
}

func copyWorkspaceFile(sourcePath string, targetPath string, perm os.FileMode) error {
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to open file %q: %w", sourcePath, err)
	}
	defer sourceFile.Close()

	targetFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return fmt.Errorf("failed to create file %q: %w", targetPath, err)
	}
	if _, err := io.Copy(targetFile, sourceFile); err != nil {
		_ = targetFile.Close()
		return fmt.Errorf("failed to copy file %q: %w", sourcePath, err)
	}
	if err := targetFile.Close(); err != nil {
		return fmt.Errorf("failed to close file %q: %w", targetPath, err)
	}
	if err := os.Chmod(targetPath, perm); err != nil {
		return fmt.Errorf("failed to set permissions on file %q: %w", targetPath, err)
	}
	return nil
}
