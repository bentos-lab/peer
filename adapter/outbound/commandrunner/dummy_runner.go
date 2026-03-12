package commandrunner

import (
	"context"
	"fmt"
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
	step, err := d.consumeStep(name, args...)
	if err != nil {
		return Result{}, err
	}
	return step.Result, step.Err
}

// RunStream executes one scripted step and streams scripted chunks.
func (d *DummyCommandRunner) RunStream(_ context.Context, onChunk func(StreamChunk), name string, args ...string) (Result, error) {
	step, err := d.consumeStep(name, args...)
	if err != nil {
		return Result{}, err
	}

	events := step.Stream
	if len(events) == 0 {
		if len(step.Result.Stdout) > 0 {
			events = append(events, StreamChunk{
				Type: StreamTypeStdout,
				Data: append([]byte(nil), step.Result.Stdout...),
			})
		}
		if len(step.Result.Stderr) > 0 {
			events = append(events, StreamChunk{
				Type: StreamTypeStderr,
				Data: append([]byte(nil), step.Result.Stderr...),
			})
		}
	}
	for _, chunk := range events {
		if onChunk == nil {
			continue
		}
		onChunk(StreamChunk{
			Type: chunk.Type,
			Data: append([]byte(nil), chunk.Data...),
		})
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
