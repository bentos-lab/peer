package host

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"bentos-backend/adapter/outbound/commandrunner"
	"bentos-backend/domain"
	"github.com/stretchr/testify/require"
)

func TestHostOpencodeAgentRunRequiresTaskProviderAndModel(t *testing.T) {
	agent := NewHostOpencodeAgent("/workspace/current", commandrunner.NewDummyCommandRunner(), nil)

	_, err := agent.Run(context.Background(), "", domain.CodingAgentRunOptions{
		Provider: "openai",
		Model:    "gpt-4o-mini",
	})
	require.EqualError(t, err, "task is required")

	_, err = agent.Run(context.Background(), "task", domain.CodingAgentRunOptions{
		Provider: "",
		Model:    "gpt-4o-mini",
	})
	require.EqualError(t, err, "provider is required")

	_, err = agent.Run(context.Background(), "task", domain.CodingAgentRunOptions{
		Provider: "openai",
		Model:    "",
	})
	require.EqualError(t, err, "model is required")
}

func TestHostOpencodeAgentRunBuildsCommandAndParsesFinalAssistantText(t *testing.T) {
	runner := commandrunner.NewDummyCommandRunner()
	logger := &opencodeAgentTestLogger{}
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "opencode",
			Args: []string{
				"run",
				"--format", "json",
				"--dir", "/workspace/current",
				"--model", "openai/gpt-4o-mini",
				"Task abc",
			},
		},
		Result: commandrunner.Result{
			Stdout: []byte(
				"{\"type\":\"assistant_message\",\"message\":{\"role\":\"assistant\",\"content\":[{\"type\":\"text\",\"text\":\"First output\"}]}}\n" +
					"{\"type\":\"assistant_message\",\"message\":{\"role\":\"assistant\",\"content\":[{\"type\":\"text\",\"text\":\"Final output\"}]}}\n",
			),
		},
	})
	agent := NewHostOpencodeAgent("/workspace/current", runner, logger)

	result, err := agent.Run(context.Background(), "Task abc", domain.CodingAgentRunOptions{
		Provider: "openai",
		Model:    "gpt-4o-mini",
	})
	require.NoError(t, err)
	require.Equal(t, "Final output", result.Text)
	require.NoError(t, runner.VerifyDone())
	require.NotEmpty(t, logger.traceLogs)
	traceContent := strings.Join(logger.traceLogs, "\n")
	require.Contains(t, traceContent, `action="agent produced assistant message"`)
	require.Contains(t, traceContent, `action="agent finalized assistant transcript"`)
	require.Contains(t, traceContent, `source=assistant_message`)
	require.Contains(t, traceContent, `content="Final output"`)
	require.NotContains(t, traceContent, "{\"type\":")
}

func TestHostOpencodeAgentRunParsesDeltaFallback(t *testing.T) {
	runner := commandrunner.NewDummyCommandRunner()
	logger := &opencodeAgentTestLogger{}
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "opencode",
			Args: []string{
				"run",
				"--format", "json",
				"--dir", "/workspace/current",
				"--model", "openai/gpt-4o-mini",
				"Task abc",
			},
		},
		Result: commandrunner.Result{
			Stdout: []byte(
				"{\"type\":\"assistant_delta\",\"delta\":\"Hello\"}\n" +
					"{\"type\":\"assistant_delta\",\"delta\":\" world\"}\n",
			),
		},
	})
	agent := NewHostOpencodeAgent("/workspace/current", runner, logger)

	result, err := agent.Run(context.Background(), "Task abc", domain.CodingAgentRunOptions{
		Provider: "openai",
		Model:    "gpt-4o-mini",
	})
	require.NoError(t, err)
	require.Equal(t, "Hello world", result.Text)
	traceContent := strings.Join(logger.traceLogs, "\n")
	require.Contains(t, traceContent, `action="agent streamed assistant delta"`)
	require.Contains(t, traceContent, `source=assistant_delta`)
}

func TestHostOpencodeAgentRunParsesOpencodeTextPartEvents(t *testing.T) {
	runner := commandrunner.NewDummyCommandRunner()
	logger := &opencodeAgentTestLogger{}
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "opencode",
			Args: []string{
				"run",
				"--format", "json",
				"--dir", "/workspace/current",
				"--model", "openai/gpt-4o-mini",
				"Task abc",
			},
		},
		Result: commandrunner.Result{
			Stdout: []byte(
				"{\"type\":\"step_start\",\"timestamp\":1773032899268,\"sessionID\":\"ses_1\",\"part\":{\"id\":\"p1\",\"type\":\"step-start\"}}\n" +
					"{\"type\":\"text\",\"timestamp\":1773032899426,\"sessionID\":\"ses_1\",\"part\":{\"id\":\"p2\",\"type\":\"text\",\"text\":\"Hello! How can I help you today?\"}}\n" +
					"{\"type\":\"step_finish\",\"timestamp\":1773032899450,\"sessionID\":\"ses_1\",\"part\":{\"id\":\"p3\",\"type\":\"step-finish\",\"reason\":\"stop\"}}\n",
			),
		},
	})
	agent := NewHostOpencodeAgent("/workspace/current", runner, logger)

	result, err := agent.Run(context.Background(), "Task abc", domain.CodingAgentRunOptions{
		Provider: "openai",
		Model:    "gpt-4o-mini",
	})
	require.NoError(t, err)
	require.Equal(t, "Hello! How can I help you today?", result.Text)
	require.NoError(t, runner.VerifyDone())
	traceContent := strings.Join(logger.traceLogs, "\n")
	require.Contains(t, traceContent, `action="agent produced assistant message"`)
	require.Contains(t, traceContent, `action="agent finalized assistant transcript"`)
}

func TestHostOpencodeAgentRunReturnsErrorWhenJSONIsMalformed(t *testing.T) {
	runner := commandrunner.NewDummyCommandRunner()
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "opencode",
			Args: []string{
				"run",
				"--format", "json",
				"--dir", "/workspace/current",
				"--model", "openai/gpt-4o-mini",
				"Task abc",
			},
		},
		Result: commandrunner.Result{
			Stdout: []byte("{invalid-json}\n"),
		},
	})
	agent := NewHostOpencodeAgent("/workspace/current", runner, nil)

	_, err := agent.Run(context.Background(), "Task abc", domain.CodingAgentRunOptions{
		Provider: "openai",
		Model:    "gpt-4o-mini",
	})
	require.ErrorContains(t, err, "failed to parse opencode json output")
}

func TestHostOpencodeAgentRunReturnsErrorWhenNoAssistantOutput(t *testing.T) {
	runner := commandrunner.NewDummyCommandRunner()
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "opencode",
			Args: []string{
				"run",
				"--format", "json",
				"--dir", "/workspace/current",
				"--model", "openai/gpt-4o-mini",
				"Task abc",
			},
		},
		Result: commandrunner.Result{
			Stdout: []byte("{\"type\":\"system\",\"text\":\"done\"}\n"),
		},
	})
	agent := NewHostOpencodeAgent("/workspace/current", runner, nil)

	_, err := agent.Run(context.Background(), "Task abc", domain.CodingAgentRunOptions{
		Provider: "openai",
		Model:    "gpt-4o-mini",
	})
	require.EqualError(t, err, "no assistant output found in opencode response")
}

func TestHostOpencodeAgentRunTruncatesTranscriptTrace(t *testing.T) {
	runner := commandrunner.NewDummyCommandRunner()
	logger := &opencodeAgentTestLogger{}
	longText := strings.Repeat("a", opencodeTraceTranscriptMaxChars+32)
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "opencode",
			Args: []string{
				"run",
				"--format", "json",
				"--dir", "/workspace/current",
				"--model", "openai/gpt-4o-mini",
				"Task abc",
			},
		},
		Result: commandrunner.Result{
			Stdout: []byte(fmt.Sprintf("{\"type\":\"assistant_message\",\"message\":{\"role\":\"assistant\",\"content\":[{\"type\":\"text\",\"text\":\"%s\"}]}}\n", longText)),
		},
	})
	agent := NewHostOpencodeAgent("/workspace/current", runner, logger)

	result, err := agent.Run(context.Background(), "Task abc", domain.CodingAgentRunOptions{
		Provider: "openai",
		Model:    "gpt-4o-mini",
	})
	require.NoError(t, err)
	require.Equal(t, longText, result.Text)

	foundTranscript := false
	for _, logLine := range logger.traceLogs {
		if !strings.Contains(logLine, `action="agent finalized assistant transcript"`) {
			continue
		}
		foundTranscript = true
		require.Contains(t, logLine, "[truncated ...]")
		require.Contains(t, logLine, fmt.Sprintf("chars=%d", len(longText)))
	}
	require.True(t, foundTranscript)
}

func TestHostOpencodeAgentRunParsesFragmentedStreamChunks(t *testing.T) {
	runner := commandrunner.NewDummyCommandRunner()
	logger := &opencodeAgentTestLogger{}
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "opencode",
			Args: []string{
				"run",
				"--format", "json",
				"--dir", "/workspace/current",
				"--model", "openai/gpt-4o-mini",
				"Task abc",
			},
		},
		Stream: []commandrunner.StreamChunk{
			{Type: commandrunner.StreamTypeStdout, Data: []byte("{\"type\":\"assistant_delta\",\"delta\":\"Hello\"}\n{\"type\":\"assistant_mes")},
			{Type: commandrunner.StreamTypeStdout, Data: []byte("sage\",\"message\":{\"role\":\"assistant\",\"content\":[{\"type\":\"text\",\"text\":\"Final text\"}]}}\n")},
		},
	})
	agent := NewHostOpencodeAgent("/workspace/current", runner, logger)

	result, err := agent.Run(context.Background(), "Task abc", domain.CodingAgentRunOptions{
		Provider: "openai",
		Model:    "gpt-4o-mini",
	})
	require.NoError(t, err)
	require.Equal(t, "Final text", result.Text)
}

func TestHostOpencodeAgentRunWarnLogsStderrStream(t *testing.T) {
	runner := commandrunner.NewDummyCommandRunner()
	logger := &opencodeAgentTestLogger{}
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "opencode",
			Args: []string{
				"run",
				"--format", "json",
				"--dir", "/workspace/current",
				"--model", "openai/gpt-4o-mini",
				"Task abc",
			},
		},
		Stream: []commandrunner.StreamChunk{
			{Type: commandrunner.StreamTypeStderr, Data: []byte("warning line 1\nwarning ")},
			{Type: commandrunner.StreamTypeStderr, Data: []byte("line 2\n")},
			{Type: commandrunner.StreamTypeStdout, Data: []byte("{\"type\":\"assistant_message\",\"message\":{\"role\":\"assistant\",\"content\":[{\"type\":\"text\",\"text\":\"Final output\"}]}}\n")},
		},
	})
	agent := NewHostOpencodeAgent("/workspace/current", runner, logger)

	result, err := agent.Run(context.Background(), "Task abc", domain.CodingAgentRunOptions{
		Provider: "openai",
		Model:    "gpt-4o-mini",
	})
	require.NoError(t, err)
	require.Equal(t, "Final output", result.Text)
	require.Equal(t, []string{
		"coding-agent opencode stderr: warning line 1",
		"coding-agent opencode stderr: warning line 2",
	}, logger.warnLogs)
}

func TestHostOpencodeAgentRunTracesToolUseActionWithoutOutputPayload(t *testing.T) {
	runner := commandrunner.NewDummyCommandRunner()
	logger := &opencodeAgentTestLogger{}
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "opencode",
			Args: []string{
				"run",
				"--format", "json",
				"--dir", "/workspace/current",
				"--model", "openai/gpt-4o-mini",
				"Task abc",
			},
		},
		Result: commandrunner.Result{
			Stdout: []byte(
				"{\"type\":\"tool_use\",\"part\":{\"tool\":\"read\",\"state\":{\"status\":\"completed\",\"input\":{\"filePath\":\"/workspace/current/README.md\"},\"output\":\"VERY_LONG_TOOL_RESULT_BODY\"}}}\n" +
					"{\"type\":\"assistant_message\",\"message\":{\"role\":\"assistant\",\"content\":[{\"type\":\"text\",\"text\":\"Done\"}]}}\n",
			),
		},
	})
	agent := NewHostOpencodeAgent("/workspace/current", runner, logger)

	result, err := agent.Run(context.Background(), "Task abc", domain.CodingAgentRunOptions{
		Provider: "openai",
		Model:    "gpt-4o-mini",
	})
	require.NoError(t, err)
	require.Equal(t, "Done", result.Text)

	traceContent := strings.Join(logger.traceLogs, "\n")
	require.Contains(t, traceContent, `action="agent read file /workspace/current/README.md"`)
	require.NotContains(t, traceContent, "VERY_LONG_TOOL_RESULT_BODY")
}

type opencodeAgentTestLogger struct {
	traceLogs []string
	warnLogs  []string
}

func (l *opencodeAgentTestLogger) Tracef(format string, args ...any) {
	l.traceLogs = append(l.traceLogs, fmt.Sprintf(format, args...))
}

func (*opencodeAgentTestLogger) Debugf(string, ...any) {}

func (*opencodeAgentTestLogger) Infof(string, ...any) {}

func (l *opencodeAgentTestLogger) Warnf(format string, args ...any) {
	l.warnLogs = append(l.warnLogs, fmt.Sprintf(format, args...))
}

func (*opencodeAgentTestLogger) Errorf(string, ...any) {}
