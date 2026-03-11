package safetysanitizer

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"

	"bentos-backend/domain"
	"bentos-backend/usecase"
	"bentos-backend/usecase/contracts"
)

//go:embed system.md
var sanitizerSystemPromptTemplateRaw string

// Options controls sanitizer behavior.
type Options struct {
	EnforceReadOnly bool
}

// Sanitizer cleans and classifies user prompts.
type Sanitizer struct {
	llm          contracts.LLMGenerator
	systemPrompt string
}

// NewSanitizer creates a safety sanitizer.
func NewSanitizer(llm contracts.LLMGenerator, options Options) (*Sanitizer, error) {
	if llm == nil {
		return nil, fmt.Errorf("sanitizer llm must not be nil")
	}
	systemPrompt, err := renderSystemPrompt(options)
	if err != nil {
		return nil, err
	}
	return &Sanitizer{
		llm:          llm,
		systemPrompt: systemPrompt,
	}, nil
}

// Sanitize rewrites and classifies the prompt.
func (s *Sanitizer) Sanitize(ctx context.Context, prompt string) (usecase.SanitizedPrompt, error) {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return usecase.SanitizedPrompt{
			Status:         domain.PromptSafetyStatusUnsupported,
			RefusalMessage: "Thanks for the prompt. I need a concrete prompt to help.",
		}, nil
	}

	output, err := s.llm.GenerateJSON(ctx, contracts.GenerateParams{
		SystemPrompt: s.systemPrompt,
		Messages:     []string{prompt},
	}, sanitizerResponseSchema())
	if err != nil {
		return usecase.SanitizedPrompt{}, err
	}

	var decoded struct {
		Status          string `json:"status"`
		SanitizedPrompt string `json:"sanitized_prompt"`
		RefusalMessage  string `json:"refusal_message"`
	}
	if err := decodeSanitizerOutput(output, &decoded); err != nil {
		return usecase.SanitizedPrompt{}, err
	}

	status := normalizeStatus(decoded.Status)
	result := usecase.SanitizedPrompt{
		Status:          status,
		SanitizedPrompt: strings.TrimSpace(decoded.SanitizedPrompt),
		RefusalMessage:  strings.TrimSpace(decoded.RefusalMessage),
	}

	if result.Status != domain.PromptSafetyStatusOK && result.RefusalMessage == "" {
		result.RefusalMessage = "Thanks for the prompt. I can't safely help with that request."
	}
	if result.Status == domain.PromptSafetyStatusOK && result.SanitizedPrompt == "" {
		result.Status = domain.PromptSafetyStatusUnsupported
		result.RefusalMessage = "Thanks for the prompt. I need more details to help."
	}
	return result, nil
}

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
