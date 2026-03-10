package commandrunner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
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

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return Result{}, err
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return Result{}, err
	}

	if err := cmd.Start(); err != nil {
		return Result{}, err
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	var wg sync.WaitGroup
	var streamErrMu sync.Mutex
	var streamErr error

	readPipe := func(pipe io.Reader, streamType StreamType, target *bytes.Buffer) {
		defer wg.Done()

		buf := make([]byte, 4096)
		for {
			n, readErr := pipe.Read(buf)
			if n > 0 {
				chunk := append([]byte(nil), buf[:n]...)
				target.Write(chunk)
				if onChunk != nil {
					onChunk(StreamChunk{
						Type: streamType,
						Data: chunk,
					})
				}
			}
			if readErr == nil {
				continue
			}
			if readErr == io.EOF {
				return
			}
			if errors.Is(readErr, os.ErrClosed) {
				return
			}

			streamErrMu.Lock()
			if streamErr == nil {
				streamErr = fmt.Errorf("failed to read %s stream: %w", streamType, readErr)
			}
			streamErrMu.Unlock()
			return
		}
	}

	wg.Add(2)
	go readPipe(stdoutPipe, StreamTypeStdout, &stdout)
	go readPipe(stderrPipe, StreamTypeStderr, &stderr)

	waitErr := cmd.Wait()
	wg.Wait()

	result := Result{
		Stdout: stdout.Bytes(),
		Stderr: stderr.Bytes(),
	}

	if waitErr != nil {
		return result, waitErr
	}

	streamErrMu.Lock()
	defer streamErrMu.Unlock()
	if streamErr != nil {
		return result, streamErr
	}

	return result, nil
}
