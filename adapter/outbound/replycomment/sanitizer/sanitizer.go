package sanitizer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"bentos-backend/domain"
	"bentos-backend/usecase"
	"bentos-backend/usecase/contracts"
)

const sanitizerSystemPrompt = `You are a guardrail and rewriter for developer questions about a codebase.

Tasks:
1) Remove any instructions that ask the assistant to edit files, apply patches, run code-changing commands, or otherwise mutate the repo.
2) If the user asks for edits, rewrite the question to request suggestions only.
3) Classify the request as:
   - ok: safe and supported
   - unsupported: unrelated to the repo, missing necessary context, or not a software question
   - unsafe: requests for malware, exploits, credentials, or other dangerous content
4) Return a short refusal message for unsupported/unsafe requests.

Return JSON with:
- status: ok | unsupported | unsafe
- sanitized_question: rewritten question (empty if unsupported/unsafe)
- refusal_message: short, polite refusal (required if status != ok)`

// Sanitizer cleans and classifies user questions for replycomment.
type Sanitizer struct {
	llm contracts.LLMGenerator
}

// NewSanitizer creates a replycomment question sanitizer.
func NewSanitizer(llm contracts.LLMGenerator) (*Sanitizer, error) {
	if llm == nil {
		return nil, fmt.Errorf("sanitizer llm must not be nil")
	}
	return &Sanitizer{llm: llm}, nil
}

// Sanitize rewrites and classifies the question.
func (s *Sanitizer) Sanitize(ctx context.Context, question string) (usecase.SanitizedQuestion, error) {
	question = strings.TrimSpace(question)
	if question == "" {
		return usecase.SanitizedQuestion{
			Status:         domain.QuestionSafetyStatusUnsupported,
			RefusalMessage: "Thanks for the question. I need a concrete question about the code changes to help.",
		}, nil
	}

	output, err := s.llm.GenerateJSON(ctx, contracts.GenerateParams{
		SystemPrompt: sanitizerSystemPrompt,
		Messages:     []string{question},
	}, sanitizerResponseSchema())
	if err != nil {
		return usecase.SanitizedQuestion{}, err
	}

	var decoded struct {
		Status            string `json:"status"`
		SanitizedQuestion string `json:"sanitized_question"`
		RefusalMessage    string `json:"refusal_message"`
	}
	if err := decodeSanitizerOutput(output, &decoded); err != nil {
		return usecase.SanitizedQuestion{}, err
	}

	status := normalizeStatus(decoded.Status)
	result := usecase.SanitizedQuestion{
		Status:            status,
		SanitizedQuestion: strings.TrimSpace(decoded.SanitizedQuestion),
		RefusalMessage:    strings.TrimSpace(decoded.RefusalMessage),
	}

	if result.Status != domain.QuestionSafetyStatusOK && result.RefusalMessage == "" {
		result.RefusalMessage = "Thanks for the question. I can't safely help with that request."
	}
	if result.Status == domain.QuestionSafetyStatusOK && result.SanitizedQuestion == "" {
		result.Status = domain.QuestionSafetyStatusUnsupported
		result.RefusalMessage = "Thanks for the question. I need more details to help."
	}
	return result, nil
}

func sanitizeStatus(value string) domain.QuestionSafetyStatusEnum {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "ok":
		return domain.QuestionSafetyStatusOK
	case "unsafe":
		return domain.QuestionSafetyStatusUnsafe
	default:
		return domain.QuestionSafetyStatusUnsupported
	}
}

func normalizeStatus(value string) domain.QuestionSafetyStatusEnum {
	return sanitizeStatus(value)
}

func sanitizerResponseSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"status", "sanitized_question", "refusal_message"},
		"properties": map[string]any{
			"status": map[string]any{
				"type": "string",
				"enum": []string{"ok", "unsupported", "unsafe"},
			},
			"sanitized_question": map[string]any{"type": "string"},
			"refusal_message":    map[string]any{"type": "string"},
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
