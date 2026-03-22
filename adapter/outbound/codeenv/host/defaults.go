package host

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bentos-lab/peer/adapter/outbound/commandrunner"
	"github.com/bentos-lab/peer/shared/logger/stdlogger"
	"github.com/bentos-lab/peer/usecase"
)

type hostDefaults struct {
	runner      commandrunner.Runner
	agentRunner commandrunner.StreamRunner
	getwd       func() (string, error)
	makeTempDir func() (string, error)
	logger      usecase.Logger
}

func resolveHostDefaults(
	runner commandrunner.Runner,
	agentRunner commandrunner.StreamRunner,
	getwd func() (string, error),
	makeTempDir func() (string, error),
	logger usecase.Logger,
) hostDefaults {
	if runner == nil {
		runner = commandrunner.NewOSCommandRunner()
	}
	if agentRunner == nil {
		agentRunner = commandrunner.NewOSStreamCommandRunner()
	}
	if getwd == nil {
		getwd = os.Getwd
	}
	if makeTempDir == nil {
		makeTempDir = newHostCodeEnvironmentTempDirMaker(os.UserHomeDir)
	}
	if logger == nil {
		logger = stdlogger.Nop()
	}
	return hostDefaults{
		runner:      runner,
		agentRunner: agentRunner,
		getwd:       getwd,
		makeTempDir: makeTempDir,
		logger:      logger,
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
