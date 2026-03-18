package safetysanitizer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"

	"github.com/bentos-lab/peer/domain"
)

func renderSystemPrompt(options Options) (string, error) {
	parsedTemplate, err := template.New("safety_sanitizer_system_prompt").Parse(sanitizerSystemPromptTemplateRaw)
	if err != nil {
		return "", err
	}

	var rendered bytes.Buffer
	if err := parsedTemplate.Execute(&rendered, options); err != nil {
		return "", err
	}

	return rendered.String(), nil
}

func sanitizeStatus(value string) domain.PromptSafetyStatusEnum {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "ok":
		return domain.PromptSafetyStatusOK
	case "unsafe":
		return domain.PromptSafetyStatusUnsafe
	default:
		return domain.PromptSafetyStatusUnsupported
	}
}

func normalizeStatus(value string) domain.PromptSafetyStatusEnum {
	return sanitizeStatus(value)
}

func sanitizerResponseSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"status", "sanitized_prompt", "refusal_message"},
		"properties": map[string]any{
			"status": map[string]any{
				"type": "string",
				"enum": []string{"ok", "unsupported", "unsafe"},
			},
			"sanitized_prompt": map[string]any{"type": "string"},
			"refusal_message":  map[string]any{"type": "string"},
		},
	}
}

func decodeSanitizerOutput(output map[string]any, target any) error {
	raw, err := jsonMarshal(output)
	if err != nil {
		return err
	}
	if err := jsonUnmarshal(raw, target); err != nil {
		return fmt.Errorf("invalid sanitizer output: %w", err)
	}
	return nil
}

var jsonMarshal = func(value any) ([]byte, error) { return json.Marshal(value) }
var jsonUnmarshal = func(data []byte, target any) error { return json.Unmarshal(data, target) }
