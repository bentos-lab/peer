package commandrunner

import (
	"context"
	"fmt"
	"strings"
)

// DummyCommandRunner provides scripted command results for tests.
type DummyCommandRunner struct {
	calls []CommandCall
	queue []CommandStep
}

// NewDummyCommandRunner creates a scripted in-memory command runner.
func NewDummyCommandRunner() *DummyCommandRunner {
	return &DummyCommandRunner{}
}

// Enqueue appends one expected command step to the scripted queue.
func (d *DummyCommandRunner) Enqueue(step CommandStep) {
	d.queue = append(d.queue, step)
}

// Run executes one scripted step and validates command order and arguments.
func (d *DummyCommandRunner) Run(_ context.Context, name string, args ...string) (Result, error) {
	call := CommandCall{Name: name, Args: append([]string(nil), args...)}
	d.calls = append(d.calls, call)

	if len(d.queue) == 0 {
		return Result{}, fmt.Errorf("unexpected command call: %s", formatCall(call))
	}

	step := d.queue[0]
	d.queue = d.queue[1:]

	if step.Expected.Name != name || !sameArgs(step.Expected.Args, args) {
		return Result{}, fmt.Errorf("unexpected command call: got %s, want %s", formatCall(call), formatCall(step.Expected))
	}

	return step.Result, step.Err
}

// VerifyDone returns an error when scripted steps are left unconsumed.
func (d *DummyCommandRunner) VerifyDone() error {
	if len(d.queue) == 0 {
		return nil
	}
	return fmt.Errorf("unconsumed command steps: %d", len(d.queue))
}

// Calls returns all command calls made to the dummy runner.
func (d *DummyCommandRunner) Calls() []CommandCall {
	copied := make([]CommandCall, 0, len(d.calls))
	for _, call := range d.calls {
		copied = append(copied, CommandCall{
			Name: call.Name,
			Args: append([]string(nil), call.Args...),
		})
	}
	return copied
}

func sameArgs(expected, actual []string) bool {
	if len(expected) != len(actual) {
		return false
	}
	for i := range expected {
		if expected[i] != actual[i] {
			return false
		}
	}
	return true
}

func formatCall(call CommandCall) string {
	if len(call.Args) == 0 {
		return call.Name
	}
	return call.Name + " " + strings.Join(call.Args, " ")
}
