package commandrunner

import (
	"bytes"
	"context"
	"os/exec"
)

// OSCommandRunner executes real OS commands.
type OSCommandRunner struct{}

// NewOSCommandRunner creates an OS-backed command runner.
func NewOSCommandRunner() *OSCommandRunner {
	return &OSCommandRunner{}
}

// Run executes a command and captures stdout/stderr independently.
func (r *OSCommandRunner) Run(ctx context.Context, name string, args ...string) (Result, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return Result{
		Stdout: stdout.Bytes(),
		Stderr: stderr.Bytes(),
	}, err
}
