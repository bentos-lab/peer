package host

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/bentos-lab/peer/adapter/outbound/commandrunner"
	"github.com/bentos-lab/peer/domain"
	"github.com/bentos-lab/peer/shared/toolinstall"
	"github.com/bentos-lab/peer/usecase"
	"github.com/stretchr/testify/require"
)

func TestHostOpencodeAgentRunRequiresTask(t *testing.T) {
	agent := newTestHostOpencodeAgent("/workspace/current", commandrunner.NewDummyCommandRunner(), nil)

	_, err := agent.Run(context.Background(), "", domain.CodingAgentRunOptions{
		Provider: "openai",
		Model:    "gpt-4o-mini",
	})
	require.EqualError(t, err, "task is required")
}

func newTestHostOpencodeAgent(workspaceDir string, runner commandrunner.StreamRunner, logger usecase.Logger) *HostOpencodeAgent {
	agent := NewHostOpencodeAgent(workspaceDir, runner, logger)
	agent.installer = toolinstall.NewOpencodeInstaller(&toolinstall.Deps{
		LookPath: func(name string) (string, error) {
			if name == "opencode" {
				return "/bin/opencode", nil
			}
			return "", errors.New("missing")
		},
		IsTerminal: func() bool { return false },
	})
	return agent
}

func TestHostOpencodeAgentRunAllowsEmptyProviderAndModel(t *testing.T) {
	runner := commandrunner.NewDummyCommandRunner()
	logger := &opencodeAgentTestLogger{}
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "opencode",
			Args: []string{
				"run",
				"--format", "json",
				"--dir", "/workspace/current",
				"Task abc",
			},
		},
		Result: commandrunner.Result{
			Stdout: []byte("{\"type\":\"assistant_message\",\"message\":{\"role\":\"assistant\",\"content\":[{\"type\":\"text\",\"text\":\"Done\"}]}}\n"),
		},
	})
	agent := newTestHostOpencodeAgent("/workspace/current", runner, logger)

	result, err := agent.Run(context.Background(), "Task abc", domain.CodingAgentRunOptions{})
	require.NoError(t, err)
	require.Equal(t, "Done", result.Text)
	require.Contains(t, strings.Join(logger.debugLogs, "\n"), "using default model")
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
	agent := newTestHostOpencodeAgent("/workspace/current", runner, logger)

	result, err := agent.Run(context.Background(), "Task abc", domain.CodingAgentRunOptions{
		Provider: "openai",
		Model:    "gpt-4o-mini",
	})
	require.NoError(t, err)
	require.Equal(t, "Final output", result.Text)
	require.NoError(t, runner.VerifyDone())
	require.NotEmpty(t, logger.debugLogs)
	debugContent := strings.Join(logger.debugLogs, "\n")
	require.Contains(t, debugContent, `action="agent produced assistant message"`)
	require.Contains(t, debugContent, `action="agent finalized assistant transcript"`)
	require.Contains(t, debugContent, `source=assistant_message`)
	require.Contains(t, debugContent, `content="Final output"`)
	require.NotContains(t, debugContent, "{\"type\":")
}

func TestHostOpencodeAgentRunWarnsWhenModelWithoutProvider(t *testing.T) {
	runner := commandrunner.NewDummyCommandRunner()
	logger := &opencodeAgentTestLogger{}
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "opencode",
			Args: []string{
				"run",
				"--format", "json",
				"--dir", "/workspace/current",
				"Task abc",
			},
		},
		Result: commandrunner.Result{
			Stdout: []byte("{\"type\":\"assistant_message\",\"message\":{\"role\":\"assistant\",\"content\":[{\"type\":\"text\",\"text\":\"Done\"}]}}\n"),
		},
	})
	agent := newTestHostOpencodeAgent("/workspace/current", runner, logger)

	result, err := agent.Run(context.Background(), "Task abc", domain.CodingAgentRunOptions{
		Model: "gpt-4o-mini",
	})
	require.NoError(t, err)
	require.Equal(t, "Done", result.Text)
	require.Contains(t, strings.Join(logger.warnLogs, "\n"), "provider is empty")
	require.Contains(t, strings.Join(logger.debugLogs, "\n"), "using default model")
}

func TestHostOpencodeAgentRunResolvesDefaultModelFromList(t *testing.T) {
	runner := commandrunner.NewDummyCommandRunner()
	logger := &opencodeAgentTestLogger{}
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "opencode",
			Args: []string{"models", "openai"},
		},
		Result: commandrunner.Result{
			Stdout: []byte("openai/gpt53-codex ready\nopenai/other-model ready\n"),
		},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "opencode",
			Args: []string{
				"run",
				"--format", "json",
				"--dir", "/workspace/current",
				"--model", "openai/gpt53-codex",
				"Task abc",
			},
		},
		Result: commandrunner.Result{
			Stdout: []byte("{\"type\":\"assistant_message\",\"message\":{\"role\":\"assistant\",\"content\":[{\"type\":\"text\",\"text\":\"Done\"}]}}\n"),
		},
	})
	agent := newTestHostOpencodeAgent("/workspace/current", runner, logger)

	result, err := agent.Run(context.Background(), "Task abc", domain.CodingAgentRunOptions{
		Provider: "openai",
	})
	require.NoError(t, err)
	require.Equal(t, "Done", result.Text)
	require.Contains(t, strings.Join(logger.debugLogs, "\n"), "openai/gpt53-codex")
}

func TestHostOpencodeAgentRunUsesFirstModelWhenDefaultMissing(t *testing.T) {
	runner := commandrunner.NewDummyCommandRunner()
	logger := &opencodeAgentTestLogger{}
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "opencode",
			Args: []string{"models", "openai"},
		},
		Result: commandrunner.Result{
			Stdout: []byte("openai/alpha ready\nopenai/beta ready\n"),
		},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "opencode",
			Args: []string{
				"run",
				"--format", "json",
				"--dir", "/workspace/current",
				"--model", "openai/alpha",
				"Task abc",
			},
		},
		Result: commandrunner.Result{
			Stdout: []byte("{\"type\":\"assistant_message\",\"message\":{\"role\":\"assistant\",\"content\":[{\"type\":\"text\",\"text\":\"Done\"}]}}\n"),
		},
	})
	agent := newTestHostOpencodeAgent("/workspace/current", runner, logger)

	result, err := agent.Run(context.Background(), "Task abc", domain.CodingAgentRunOptions{
		Provider: "openai",
	})
	require.NoError(t, err)
	require.Equal(t, "Done", result.Text)
}

func TestHostOpencodeAgentRunFallsBackWhenModelListEmpty(t *testing.T) {
	runner := commandrunner.NewDummyCommandRunner()
	logger := &opencodeAgentTestLogger{}
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "opencode",
			Args: []string{"models", "openai"},
		},
		Result: commandrunner.Result{
			Stdout: []byte("\n"),
		},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "opencode",
			Args: []string{
				"run",
				"--format", "json",
				"--dir", "/workspace/current",
				"Task abc",
			},
		},
		Result: commandrunner.Result{
			Stdout: []byte("{\"type\":\"assistant_message\",\"message\":{\"role\":\"assistant\",\"content\":[{\"type\":\"text\",\"text\":\"Done\"}]}}\n"),
		},
	})
	agent := newTestHostOpencodeAgent("/workspace/current", runner, logger)

	result, err := agent.Run(context.Background(), "Task abc", domain.CodingAgentRunOptions{
		Provider: "openai",
	})
	require.NoError(t, err)
	require.Equal(t, "Done", result.Text)
	require.Contains(t, strings.Join(logger.warnLogs, "\n"), "no models returned")
	require.Contains(t, strings.Join(logger.debugLogs, "\n"), "using default model")
}

func TestHostOpencodeAgentRunFallsBackWhenModelListFails(t *testing.T) {
	runner := commandrunner.NewDummyCommandRunner()
	logger := &opencodeAgentTestLogger{}
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "opencode",
			Args: []string{"models", "openai"},
		},
		Err: fmt.Errorf("boom"),
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "opencode",
			Args: []string{
				"run",
				"--format", "json",
				"--dir", "/workspace/current",
				"Task abc",
			},
		},
		Result: commandrunner.Result{
			Stdout: []byte("{\"type\":\"assistant_message\",\"message\":{\"role\":\"assistant\",\"content\":[{\"type\":\"text\",\"text\":\"Done\"}]}}\n"),
		},
	})
	agent := newTestHostOpencodeAgent("/workspace/current", runner, logger)

	result, err := agent.Run(context.Background(), "Task abc", domain.CodingAgentRunOptions{
		Provider: "openai",
	})
	require.NoError(t, err)
	require.Equal(t, "Done", result.Text)
	require.Contains(t, strings.Join(logger.warnLogs, "\n"), "failed to list models")
	require.Contains(t, strings.Join(logger.debugLogs, "\n"), "using default model")
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
	agent := newTestHostOpencodeAgent("/workspace/current", runner, logger)

	result, err := agent.Run(context.Background(), "Task abc", domain.CodingAgentRunOptions{
		Provider: "openai",
		Model:    "gpt-4o-mini",
	})
	require.NoError(t, err)
	require.Equal(t, "Hello world", result.Text)
	debugContent := strings.Join(logger.debugLogs, "\n")
	require.Contains(t, debugContent, `action="agent streamed assistant delta"`)
	require.Contains(t, debugContent, `source=assistant_delta`)
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
	agent := newTestHostOpencodeAgent("/workspace/current", runner, logger)

	result, err := agent.Run(context.Background(), "Task abc", domain.CodingAgentRunOptions{
		Provider: "openai",
		Model:    "gpt-4o-mini",
	})
	require.NoError(t, err)
	require.Equal(t, "Hello! How can I help you today?", result.Text)
	require.Equal(t, "ses_1", result.SessionID)
	require.NoError(t, runner.VerifyDone())
	debugContent := strings.Join(logger.debugLogs, "\n")
	require.Contains(t, debugContent, `action="agent produced assistant message"`)
	require.Contains(t, debugContent, `action="agent finalized assistant transcript"`)
}

func TestHostOpencodeAgentRunPassesSessionID(t *testing.T) {
	runner := commandrunner.NewDummyCommandRunner()
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{
			Name: "opencode",
			Args: []string{
				"run",
				"--format", "json",
				"--dir", "/workspace/current",
				"--session", "ses_123",
				"--model", "openai/gpt-4o-mini",
				"Task abc",
			},
		},
		Result: commandrunner.Result{
			Stdout: []byte("{\"type\":\"assistant_message\",\"message\":{\"role\":\"assistant\",\"content\":[{\"type\":\"text\",\"text\":\"Done\"}]},\"sessionID\":\"ses_123\"}\n"),
		},
	})
	agent := newTestHostOpencodeAgent("/workspace/current", runner, nil)

	result, err := agent.Run(context.Background(), "Task abc", domain.CodingAgentRunOptions{
		Provider:  "openai",
		Model:     "gpt-4o-mini",
		SessionID: "ses_123",
	})
	require.NoError(t, err)
	require.Equal(t, "Done", result.Text)
	require.Equal(t, "ses_123", result.SessionID)
	require.NoError(t, runner.VerifyDone())
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
	agent := newTestHostOpencodeAgent("/workspace/current", runner, nil)

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
	agent := newTestHostOpencodeAgent("/workspace/current", runner, nil)

	_, err := agent.Run(context.Background(), "Task abc", domain.CodingAgentRunOptions{
		Provider: "openai",
		Model:    "gpt-4o-mini",
	})
	require.EqualError(t, err, "no assistant output found in opencode response")
}

func TestHostOpencodeAgentRunTruncatesTranscriptTrace(t *testing.T) {
	runner := commandrunner.NewDummyCommandRunner()
	logger := &opencodeAgentTestLogger{}
	longText := strings.Repeat("a", opencodeDebugTranscriptMaxChars+32)
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
	agent := newTestHostOpencodeAgent("/workspace/current", runner, logger)

	result, err := agent.Run(context.Background(), "Task abc", domain.CodingAgentRunOptions{
		Provider: "openai",
		Model:    "gpt-4o-mini",
	})
	require.NoError(t, err)
	require.Equal(t, longText, result.Text)

	foundTranscript := false
	for _, logLine := range logger.debugLogs {
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
	agent := newTestHostOpencodeAgent("/workspace/current", runner, logger)

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
	agent := newTestHostOpencodeAgent("/workspace/current", runner, logger)

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
	agent := newTestHostOpencodeAgent("/workspace/current", runner, logger)

	result, err := agent.Run(context.Background(), "Task abc", domain.CodingAgentRunOptions{
		Provider: "openai",
		Model:    "gpt-4o-mini",
	})
	require.NoError(t, err)
	require.Equal(t, "Done", result.Text)

	debugContent := strings.Join(logger.debugLogs, "\n")
	require.Contains(t, debugContent, `action="agent read file /workspace/current/README.md"`)
	require.NotContains(t, debugContent, "VERY_LONG_TOOL_RESULT_BODY")
}

type opencodeAgentTestLogger struct {
	debugLogs []string
	traceLogs []string
	warnLogs  []string
}

func (l *opencodeAgentTestLogger) Tracef(format string, args ...any) {
	l.traceLogs = append(l.traceLogs, fmt.Sprintf(format, args...))
}

func (l *opencodeAgentTestLogger) Debugf(format string, args ...any) {
	l.debugLogs = append(l.debugLogs, fmt.Sprintf(format, args...))
}

func (*opencodeAgentTestLogger) Infof(string, ...any) {}

func (l *opencodeAgentTestLogger) Warnf(format string, args ...any) {
	l.warnLogs = append(l.warnLogs, fmt.Sprintf(format, args...))
}

func (*opencodeAgentTestLogger) Errorf(string, ...any) {}
