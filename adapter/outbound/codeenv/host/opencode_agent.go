package host

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"bentos-backend/adapter/outbound/commandrunner"
	"bentos-backend/domain"
	"bentos-backend/shared/logger/stdlogger"
	"bentos-backend/shared/toolinstall"
	"bentos-backend/usecase"
)

// HostOpencodeAgent runs tasks through the opencode CLI on the host machine.
type HostOpencodeAgent struct {
	workspaceDir string
	runner       commandrunner.StreamRunner
	logger       usecase.Logger
	installer    *toolinstall.Installer
}

// NewHostOpencodeAgent creates a host opencode coding agent.
func NewHostOpencodeAgent(workspaceDir string, runner commandrunner.StreamRunner, logger usecase.Logger) *HostOpencodeAgent {
	if runner == nil {
		runner = commandrunner.NewOSStreamCommandRunner()
	}
	if logger == nil {
		logger = stdlogger.Nop()
	}
	return &HostOpencodeAgent{
		workspaceDir: workspaceDir,
		runner:       runner,
		logger:       logger,
		installer:    toolinstall.NewInstaller(toolinstall.Config{}),
	}
}

// Run executes one coding task using opencode JSON output mode.
func (a *HostOpencodeAgent) Run(ctx context.Context, task string, opts domain.CodingAgentRunOptions) (domain.CodingAgentRunResult, error) {
	task = strings.TrimSpace(task)
	if task == "" {
		return domain.CodingAgentRunResult{}, fmt.Errorf("task is required")
	}

	if err := a.ensureOpencodeInstalled(ctx); err != nil {
		return domain.CodingAgentRunResult{}, err
	}

	a.logger.Tracef("Open-code task: %s", task)

	provider := strings.TrimSpace(opts.Provider)
	model := strings.TrimSpace(opts.Model)
	modelSpec, err := a.resolveModelSpec(ctx, provider, model)
	if err != nil {
		return domain.CodingAgentRunResult{}, err
	}

	if modelSpec == "" {
		a.logger.Debugf("coding-agent opencode using default model")
	} else {
		a.logger.Debugf("coding-agent opencode using model %s", modelSpec)
	}
	parser := newOpencodeJSONStreamParser(a.logger)
	stderrBuffer := newLineBuffer(func(line string) {
		if strings.TrimSpace(line) == "" {
			return
		}
		a.logger.Warnf("coding-agent opencode stderr: %s", line)
	})

	args := []string{
		"run",
		"--format",
		"json",
		"--dir",
		a.workspaceDir,
	}
	if sessionID := strings.TrimSpace(opts.SessionID); sessionID != "" {
		args = append(args, "--session", sessionID)
	}
	if modelSpec != "" {
		args = append(args, "--model", modelSpec)
	}
	args = append(args, task)
	result, err := a.runner.RunStream(
		ctx,
		func(chunk commandrunner.StreamChunk) {
			if len(chunk.Data) == 0 {
				return
			}
			switch chunk.Type {
			case commandrunner.StreamTypeStdout:
				parser.Consume(chunk.Data)
			case commandrunner.StreamTypeStderr:
				stderrBuffer.Append(chunk.Data)
			}
		},
		"opencode",
		args...,
	)
	stderrBuffer.Flush()
	if err != nil {
		return domain.CodingAgentRunResult{}, fmt.Errorf("failed to run opencode task: %w", formatCommandError(err, result))
	}

	text, err := parser.Finalize()
	if err != nil {
		return domain.CodingAgentRunResult{}, err
	}

	return domain.CodingAgentRunResult{Text: text, SessionID: parser.sessionID}, nil
}

const opencodeTraceTranscriptMaxChars = 16000

type parsedOpencodeEvent struct {
	Type   string
	Text   string
	Action string
}

type opencodeJSONStreamParser struct {
	logger                usecase.Logger
	stdoutLineBuffer      lineBuffer
	finalText             string
	assistantDelta        strings.Builder
	parsedLineCount       int
	assistantMessageCount int
	assistantDeltaCount   int
	lineNumber            int
	firstError            error
	sessionID             string
}

// Consume processes one stdout chunk from opencode in real time.
func (p *opencodeJSONStreamParser) Consume(stdoutChunk []byte) {
	if p.firstError != nil || len(stdoutChunk) == 0 {
		return
	}
	p.stdoutLineBuffer.Append(stdoutChunk)
}

// Finalize flushes any remaining buffered line and resolves final assistant text.
func (p *opencodeJSONStreamParser) Finalize() (string, error) {
	p.stdoutLineBuffer.Flush()
	if p.firstError != nil {
		return "", p.firstError
	}

	logTranscript := func(source string, text string) {
		transcriptLineCount := strings.Count(text, "\n") + 1
		truncated := truncateForTrace(text, opencodeTraceTranscriptMaxChars)
		p.logger.Tracef(
			"coding-agent trace action=%q source=%s parsed_lines=%d message_events=%d delta_events=%d chars=%d lines=%d content=%q",
			"agent finalized assistant transcript",
			source,
			p.parsedLineCount,
			p.assistantMessageCount,
			p.assistantDeltaCount,
			len(text),
			transcriptLineCount,
			truncated,
		)
	}

	p.finalText = strings.TrimSpace(p.finalText)
	if p.finalText != "" {
		logTranscript("assistant_message", p.finalText)
		return p.finalText, nil
	}

	deltaText := strings.TrimSpace(p.assistantDelta.String())
	if deltaText != "" {
		logTranscript("assistant_delta", deltaText)
		return deltaText, nil
	}

	return "", fmt.Errorf("no assistant output found in opencode response")
}

type lineBuffer struct {
	buffer      bytes.Buffer
	consumeLine func(string)
}

func (b *lineBuffer) Append(chunk []byte) {
	if len(chunk) == 0 {
		return
	}
	_, _ = b.buffer.Write(chunk)

	for {
		content := b.buffer.Bytes()
		newlineIndex := bytes.IndexByte(content, '\n')
		if newlineIndex < 0 {
			return
		}

		line := string(content[:newlineIndex])
		remaining := append([]byte(nil), content[newlineIndex+1:]...)
		b.buffer.Reset()
		_, _ = b.buffer.Write(remaining)
		if b.consumeLine != nil {
			b.consumeLine(line)
		}
	}
}

func (b *lineBuffer) Flush() {
	if b.buffer.Len() == 0 {
		return
	}
	line := b.buffer.String()
	b.buffer.Reset()
	if b.consumeLine != nil {
		b.consumeLine(line)
	}
}
