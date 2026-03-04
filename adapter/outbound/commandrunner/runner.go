package commandrunner

import "context"

// Runner executes shell commands and returns captured process output.
type Runner interface {
	Run(ctx context.Context, name string, args ...string) (Result, error)
}

// Result contains command standard streams captured from execution.
type Result struct {
	Stdout []byte
	Stderr []byte
}

// CommandCall stores a single command invocation.
type CommandCall struct {
	Name string
	Args []string
}

// CommandStep defines one scripted dummy command expectation and outcome.
type CommandStep struct {
	Expected CommandCall
	Result   Result
	Err      error
}
