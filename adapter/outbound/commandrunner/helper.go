package commandrunner

import (
	"fmt"
	"strings"
)

func (d *DummyCommandRunner) consumeStep(name string, args ...string) (CommandStep, error) {
	call := CommandCall{Name: name, Args: append([]string(nil), args...)}
	d.calls = append(d.calls, call)

	if len(d.queue) == 0 {
		return CommandStep{}, fmt.Errorf("unexpected command call: %s", formatCall(call))
	}

	step := d.queue[0]
	d.queue = d.queue[1:]

	if step.Expected.Name != name || !sameArgs(step.Expected.Args, args) {
		return CommandStep{}, fmt.Errorf("unexpected command call: got %s, want %s", formatCall(call), formatCall(step.Expected))
	}
	return step, nil
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
