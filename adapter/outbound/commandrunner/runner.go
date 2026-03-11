package commandrunner

import "context"

// Runner executes shell commands and returns captured process output.
type Runner interface {
	Run(ctx context.Context, name string, args ...string) (Result, error)
}

// TTYRunner executes shell commands attached to a terminal.
type TTYRunner interface {
	RunTTY(ctx context.Context, name string, args ...string) error
}

// StreamType identifies which command stream produced a chunk.
type StreamType string

const (
	// StreamTypeStdout identifies stdout chunks.
	StreamTypeStdout StreamType = "stdout"
	// StreamTypeStderr identifies stderr chunks.
	StreamTypeStderr StreamType = "stderr"
)

// StreamChunk contains one streamed command output chunk.
type StreamChunk struct {
	Type StreamType
	Data []byte
}

// StreamRunner executes shell commands and streams process output chunks while capturing final buffers.
type StreamRunner interface {
	RunStream(ctx context.Context, onChunk func(StreamChunk), name string, args ...string) (Result, error)
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
	Stream   []StreamChunk
	Result   Result
	Err      error
}
