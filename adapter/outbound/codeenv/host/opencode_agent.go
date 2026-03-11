package host

import (
	"bytes"
	"context"
	"encoding/json"
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

	return domain.CodingAgentRunResult{Text: text}, nil
}

func (a *HostOpencodeAgent) resolveModelSpec(ctx context.Context, provider string, model string) (string, error) {
	provider = strings.TrimSpace(provider)
	model = strings.TrimSpace(model)

	if provider == "" {
		if model != "" {
			a.logger.Warnf("coding-agent opencode provider is empty; clearing model %q", model)
		}
		return "", nil
	}

	if model == "" {
		models, err := a.listOpencodeModels(ctx, provider)
		if err != nil {
			a.logger.Warnf("coding-agent opencode failed to list models for provider %s: %v", provider, err)
			return "", nil
		}
		if len(models) == 0 {
			a.logger.Warnf("coding-agent opencode no models returned for provider %s", provider)
			return "", nil
		}
		model = selectDefaultOpencodeModel(provider, models)
	}

	if model == "" {
		return "", nil
	}
	return provider + "/" + model, nil
}

func (a *HostOpencodeAgent) listOpencodeModels(ctx context.Context, provider string) ([]string, error) {
	result, err := a.runner.RunStream(ctx, nil, "opencode", "models", provider)
	if err != nil {
		return nil, formatCommandError(err, result)
	}
	return parseOpencodeModelList(provider, string(result.Stdout)), nil
}

func selectDefaultOpencodeModel(provider string, models []string) string {
	defaultModel, ok := defaultOpencodeModels[strings.ToLower(provider)]
	if ok {
		for _, candidate := range models {
			if strings.EqualFold(candidate, defaultModel) {
				return candidate
			}
		}
	}
	return models[0]
}

func parseOpencodeModelList(provider string, stdout string) []string {
	provider = strings.TrimSpace(provider)
	providerLower := strings.ToLower(provider)
	lines := strings.Split(stdout, "\n")
	models := make([]string, 0, len(lines))

	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		model := strings.TrimSpace(fields[0])
		if model == "" {
			continue
		}
		if strings.Contains(model, "/") {
			lower := strings.ToLower(model)
			prefix := providerLower + "/"
			if providerLower != "" && strings.HasPrefix(lower, prefix) {
				model = model[len(prefix):]
			} else {
				model = model[strings.LastIndex(model, "/")+1:]
			}
		}
		model = strings.TrimSpace(model)
		if model == "" {
			continue
		}
		models = append(models, model)
	}
	return models
}

var defaultOpencodeModels = map[string]string{
	"openai":    "gpt-5.3-codex",
	"anthropic": "claude-sonnet-4-6",
	"gemini":    "gemini-3-pro-preview",
	"google":    "gemini-3-pro-preview",
}

func (a *HostOpencodeAgent) ensureOpencodeInstalled(ctx context.Context) error {
	if a.installer == nil {
		a.installer = toolinstall.NewInstaller(toolinstall.Config{})
	}
	return a.installer.EnsureOpencodeInstalled(ctx)
}

const opencodeTraceTranscriptMaxChars = 4096

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
}

func newOpencodeJSONStreamParser(logger usecase.Logger) *opencodeJSONStreamParser {
	if logger == nil {
		logger = stdlogger.Nop()
	}

	parser := &opencodeJSONStreamParser{
		logger: logger,
	}
	parser.stdoutLineBuffer = newLineBuffer(parser.consumeLine)
	return parser
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

func (p *opencodeJSONStreamParser) consumeLine(rawLine string) {
	if p.firstError != nil {
		return
	}
	p.lineNumber++

	line := strings.TrimSpace(rawLine)
	if line == "" {
		return
	}
	p.parsedLineCount++

	var event map[string]any
	if err := json.Unmarshal([]byte(line), &event); err != nil {
		p.firstError = fmt.Errorf("failed to parse opencode json output at line %d: %w", p.lineNumber, err)
		return
	}

	parsedEvent := extractParsedOpencodeEvent(event)
	action := strings.TrimSpace(parsedEvent.Action)
	if action != "" && action != "agent started step" && !strings.HasPrefix(action, "agent finished step") {
		p.logger.Tracef("coding-agent trace action=%q line=%d", parsedEvent.Action, p.lineNumber)
	}

	candidate := parsedEvent.Text
	if strings.TrimSpace(candidate) == "" {
		return
	}

	if parsedEvent.Type == "assistant_delta" {
		p.assistantDeltaCount++
		p.assistantDelta.WriteString(candidate)
		p.logger.Tracef(
			"coding-agent trace action=%q line=%d index=%d chars=%d",
			"agent streamed assistant delta",
			p.lineNumber,
			p.assistantDeltaCount,
			len(candidate),
		)
		return
	}

	p.assistantMessageCount++
	p.finalText = candidate
	p.logger.Tracef(
		"coding-agent trace action=%q line=%d index=%d chars=%d",
		"agent produced assistant message",
		p.lineNumber,
		p.assistantMessageCount,
		len(candidate),
	)
}

type lineBuffer struct {
	buffer      bytes.Buffer
	consumeLine func(string)
}

func newLineBuffer(consumeLine func(string)) lineBuffer {
	return lineBuffer{
		consumeLine: consumeLine,
	}
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

func extractParsedOpencodeEvent(event map[string]any) parsedOpencodeEvent {
	eventType, _ := event["type"].(string)
	eventType = strings.ToLower(strings.TrimSpace(eventType))

	switch eventType {
	case "step_start":
		return parsedOpencodeEvent{
			Type:   eventType,
			Action: "agent started step",
		}
	case "step_finish":
		action := "agent finished step"
		if reason := extractStepFinishReason(event); reason != "" {
			action = fmt.Sprintf("agent finished step reason=%s", reason)
		}
		return parsedOpencodeEvent{
			Type:   eventType,
			Action: action,
		}
	case "tool_use":
		action := extractToolUseAction(event)
		if action == "" {
			action = "agent used tool"
		}
		return parsedOpencodeEvent{
			Type:   eventType,
			Action: action,
		}
	}

	if eventType == "text" {
		if part, ok := event["part"].(map[string]any); ok {
			partType, _ := part["type"].(string)
			if strings.EqualFold(strings.TrimSpace(partType), "text") {
				if text, _ := part["text"].(string); strings.TrimSpace(text) != "" {
					return parsedOpencodeEvent{
						Type:   "assistant_message",
						Text:   text,
						Action: "agent produced assistant message",
					}
				}
			}
		}
	}

	if role, _ := event["role"].(string); strings.EqualFold(strings.TrimSpace(role), "assistant") {
		if content := extractTextFromValue(event["content"]); strings.TrimSpace(content) != "" {
			return parsedOpencodeEvent{
				Type:   "assistant_message",
				Text:   content,
				Action: "agent produced assistant message",
			}
		}
		if text := extractTextFromValue(event["text"]); strings.TrimSpace(text) != "" {
			return parsedOpencodeEvent{
				Type:   "assistant_message",
				Text:   text,
				Action: "agent produced assistant message",
			}
		}
	}

	if message, ok := event["message"].(map[string]any); ok {
		if role, _ := message["role"].(string); strings.EqualFold(strings.TrimSpace(role), "assistant") {
			if content := extractTextFromValue(message["content"]); strings.TrimSpace(content) != "" {
				return parsedOpencodeEvent{
					Type:   "assistant_message",
					Text:   content,
					Action: "agent produced assistant message",
				}
			}
			if text := extractTextFromValue(message["text"]); strings.TrimSpace(text) != "" {
				return parsedOpencodeEvent{
					Type:   "assistant_message",
					Text:   text,
					Action: "agent produced assistant message",
				}
			}
		}
	}

	if strings.Contains(eventType, "assistant") && strings.Contains(eventType, "delta") {
		if delta, _ := event["delta"].(string); strings.TrimSpace(delta) != "" {
			return parsedOpencodeEvent{
				Type:   "assistant_delta",
				Text:   delta,
				Action: "agent streamed assistant delta",
			}
		}
	}

	return parsedOpencodeEvent{Type: "other"}
}

func extractStepFinishReason(event map[string]any) string {
	if reason, _ := event["reason"].(string); strings.TrimSpace(reason) != "" {
		return strings.TrimSpace(reason)
	}
	part, _ := event["part"].(map[string]any)
	if part == nil {
		return ""
	}
	if reason, _ := part["reason"].(string); strings.TrimSpace(reason) != "" {
		return strings.TrimSpace(reason)
	}
	return ""
}

func extractToolUseAction(event map[string]any) string {
	part, _ := event["part"].(map[string]any)
	if part == nil {
		return ""
	}

	toolName, _ := part["tool"].(string)
	toolName = strings.TrimSpace(toolName)
	if toolName == "" {
		return ""
	}

	input := map[string]any{}
	if state, ok := part["state"].(map[string]any); ok {
		if typedInput, ok := state["input"].(map[string]any); ok {
			input = typedInput
		}
	}

	filePath := extractFirstNonEmptyString(input, "filePath", "path", "filename", "file")
	command := extractFirstNonEmptyString(input, "command", "cmd", "script")
	command = truncateForTrace(command, 256)

	switch strings.ToLower(toolName) {
	case "read":
		if filePath != "" {
			return fmt.Sprintf("agent read file %s", filePath)
		}
		return "agent read file"
	case "edit", "write", "replace", "patch", "multi_edit":
		if filePath != "" {
			return fmt.Sprintf("agent edited file %s", filePath)
		}
		return "agent edited file"
	case "bash", "shell", "run", "command", "exec", "execute", "terminal":
		if command != "" {
			return fmt.Sprintf("agent ran command %q", command)
		}
		return "agent ran command"
	default:
		if filePath != "" {
			return fmt.Sprintf("agent used tool %s on file %s", toolName, filePath)
		}
		if command != "" {
			return fmt.Sprintf("agent used tool %s with command %q", toolName, command)
		}
		return fmt.Sprintf("agent used tool %s", toolName)
	}
}

func extractFirstNonEmptyString(source map[string]any, keys ...string) string {
	for _, key := range keys {
		value := extractTextFromValue(source[key])
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func extractTextFromValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			part := strings.TrimSpace(extractTextFromValue(item))
			if part != "" {
				parts = append(parts, part)
			}
		}
		return strings.Join(parts, "\n")
	case map[string]any:
		if text, _ := typed["text"].(string); strings.TrimSpace(text) != "" {
			return text
		}
		if content := extractTextFromValue(typed["content"]); strings.TrimSpace(content) != "" {
			return content
		}
		if delta, _ := typed["delta"].(string); strings.TrimSpace(delta) != "" {
			return delta
		}
	}
	return ""
}

func truncateForTrace(value string, maxChars int) string {
	if maxChars <= 0 {
		return ""
	}
	if len(value) <= maxChars {
		return value
	}
	return strings.TrimSpace(value[:maxChars]) + " [truncated ...]"
}
