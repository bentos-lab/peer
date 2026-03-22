package host

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bentos-lab/peer/shared/logger/stdlogger"
	"github.com/bentos-lab/peer/usecase"
)

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
	if sessionID, ok := event["sessionID"].(string); ok {
		sessionID = strings.TrimSpace(sessionID)
		if sessionID != "" {
			p.sessionID = sessionID
		}
	}

	parsedEvent := extractParsedOpencodeEvent(event)
	action := strings.TrimSpace(parsedEvent.Action)
	if action != "" && action != "agent started step" && !strings.HasPrefix(action, "agent finished step") {
		p.logger.Debugf("coding-agent debug action=%q line=%d", parsedEvent.Action, p.lineNumber)
	}

	candidate := parsedEvent.Text
	if strings.TrimSpace(candidate) == "" {
		return
	}

	if parsedEvent.Type == "assistant_delta" {
		p.assistantDeltaCount++
		p.assistantDelta.WriteString(candidate)
		p.logger.Debugf(
			"coding-agent debug action=%q line=%d index=%d chars=%d",
			"agent streamed assistant delta",
			p.lineNumber,
			p.assistantDeltaCount,
			len(candidate),
		)
		return
	}

	p.assistantMessageCount++
	p.finalText = candidate
	p.logger.Debugf(
		"coding-agent debug action=%q line=%d index=%d chars=%d",
		"agent produced assistant message",
		p.lineNumber,
		p.assistantMessageCount,
		len(candidate),
	)
}

func newLineBuffer(consumeLine func(string)) lineBuffer {
	return lineBuffer{
		consumeLine: consumeLine,
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
	command = truncateForDebug(command, 256)

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

func truncateForDebug(value string, maxChars int) string {
	if maxChars <= 0 {
		return ""
	}
	if len(value) <= maxChars {
		return value
	}
	return strings.TrimSpace(value[:maxChars]) + " [truncated ...]"
}
