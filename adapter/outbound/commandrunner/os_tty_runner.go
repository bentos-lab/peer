package commandrunner

import (
	"context"
	"os"
	"os/exec"
)

// OSTTYCommandRunner executes OS commands attached to the current terminal.
type OSTTYCommandRunner struct{}

// NewOSTTYCommandRunner creates an OS-backed TTY command runner.
func NewOSTTYCommandRunner() *OSTTYCommandRunner {
	return &OSTTYCommandRunner{}
}

// RunTTY executes a command with stdin/stdout/stderr bound to the current terminal.
func (r *OSTTYCommandRunner) RunTTY(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
