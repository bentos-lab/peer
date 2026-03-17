package commandrunner

import (
	"bytes"
	"context"
	"os/exec"
	"sync"
)

// OSStreamCommandRunner executes OS commands and emits output chunks while the process is running.
type OSStreamCommandRunner struct{}

// NewOSStreamCommandRunner creates an OS-backed streaming command runner.
func NewOSStreamCommandRunner() *OSStreamCommandRunner {
	return &OSStreamCommandRunner{}
}

// RunStream executes a command, emits stream chunks, and returns captured stdout/stderr buffers.
func (r *OSStreamCommandRunner) RunStream(ctx context.Context, onChunk func(StreamChunk), name string, args ...string) (Result, error) {
	cmd := exec.CommandContext(ctx, name, args...)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = newStreamWriter(&stdout, StreamTypeStdout, onChunk)
	cmd.Stderr = newStreamWriter(&stderr, StreamTypeStderr, onChunk)

	waitErr := cmd.Run()

	result := Result{
		Stdout: stdout.Bytes(),
		Stderr: stderr.Bytes(),
	}

	if waitErr != nil {
		return result, waitErr
	}

	return result, nil
}

type streamWriter struct {
	mu         sync.Mutex
	buffer     *bytes.Buffer
	streamType StreamType
	onChunk    func(StreamChunk)
}

func newStreamWriter(buffer *bytes.Buffer, streamType StreamType, onChunk func(StreamChunk)) *streamWriter {
	return &streamWriter{
		buffer:     buffer,
		streamType: streamType,
		onChunk:    onChunk,
	}
}

func (w *streamWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	chunk := append([]byte(nil), p...)

	w.mu.Lock()
	_, _ = w.buffer.Write(chunk)
	w.mu.Unlock()

	if w.onChunk != nil {
		w.onChunk(StreamChunk{
			Type: w.streamType,
			Data: chunk,
		})
	}

	return len(p), nil
}
