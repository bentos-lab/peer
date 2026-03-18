package safetysanitizer

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	"github.com/bentos-lab/peer/domain"
	"github.com/bentos-lab/peer/usecase"
	"github.com/bentos-lab/peer/usecase/contracts"
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
