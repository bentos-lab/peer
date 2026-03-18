package llm

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bentos-lab/peer/domain"
	"github.com/bentos-lab/peer/shared/logger/stdlogger"
	"github.com/bentos-lab/peer/usecase"
	"github.com/bentos-lab/peer/usecase/contracts"
)

//go:embed review_system.md
var reviewSystemPromptTemplateRaw string

//go:embed review_input.md
var reviewUserPromptTemplateRaw string

// Reviewer implements usecase.LLMReviewer via a generic LLM generator.
type Reviewer struct {
	generator contracts.LLMGenerator
	logger    usecase.Logger
}

type reviewModelOutput struct {
	Summary  string            `json:"summary"`
	Findings []json.RawMessage `json:"findings"`
}

type reviewSystemPromptTemplateData struct {
	RulePackText string
}

type reviewUserPromptTemplateData struct {
	Title       string
	Description string
	Language    string
	Files       []reviewUserPromptFileData
}

type reviewUserPromptFileData struct {
	Path        string
	ChangedText string
}

const maxSplitFindingsPerOriginal = 3

// NewReviewer creates an outbound reviewer adapter backed by a generic LLM client.
func NewReviewer(generator contracts.LLMGenerator, logger usecase.Logger) (*Reviewer, error) {
	if generator == nil {
		return nil, fmt.Errorf("llm generator must not be nil")
	}
	if logger == nil {
		logger = stdlogger.Nop()
	}
	return &Reviewer{generator: generator, logger: logger}, nil
}

// Review generates findings from changed content by calling an LLM provider.
func (r *Reviewer) Review(ctx context.Context, payload usecase.LLMReviewPayload) (usecase.LLMReviewResult, error) {
	r.logger.Debugf("The rule pack includes %d instructions.", len(payload.RulePack.Instructions))
	if payload.Environment == nil {
		return usecase.LLMReviewResult{}, fmt.Errorf("code environment must not be nil")
	}

	changedFiles, err := payload.Environment.LoadChangedFiles(ctx, domain.CodeEnvironmentLoadOptions{
		Base: payload.Input.Base,
		Head: payload.Input.Head,
	})
	if err != nil {
		return usecase.LLMReviewResult{}, err
	}
	r.logger.Debugf("The review input includes %d changed files.", len(changedFiles))

	systemPrompt, err := renderSystemPrompt(strings.Join(payload.RulePack.Instructions, "\n\n"))
	if err != nil {
		return usecase.LLMReviewResult{}, fmt.Errorf("review: render system prompt: %w", err)
	}

	userPrompt, err := renderUserPrompt(payload.Input, changedFiles)
	if err != nil {
		return usecase.LLMReviewResult{}, fmt.Errorf("review: render user prompt: %w", err)
	}

	outputMap, err := r.generator.GenerateJSON(ctx, contracts.GenerateParams{
		SystemPrompt: systemPrompt,
		Messages:     []string{userPrompt},
	}, reviewResponseSchema())
	if err != nil {
		return usecase.LLMReviewResult{}, fmt.Errorf("review: generate JSON output: %w", err)
	}

	raw, err := json.Marshal(outputMap)
	if err != nil {
		return usecase.LLMReviewResult{}, fmt.Errorf("review: encode model output: %w", err)
	}

	var decoded reviewModelOutput
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return usecase.LLMReviewResult{}, fmt.Errorf("review: invalid model output: %w", err)
	}

	findings := make([]json.RawMessage, 0)
	if decoded.Findings != nil {
		findings = decoded.Findings
	}

	resultFindings := make([]domain.Finding, 0, len(findings))
	for _, findingRaw := range findings {
		var finding domain.Finding
		if err := json.Unmarshal(findingRaw, &finding); err != nil {
			return usecase.LLMReviewResult{}, fmt.Errorf("review: invalid finding format: %w", err)
		}
		if finding.StartLine <= 0 || finding.EndLine <= 0 || finding.StartLine > finding.EndLine {
			return usecase.LLMReviewResult{}, fmt.Errorf(
				"review: invalid finding range for %q: start line is %d and end line is %d",
				finding.FilePath,
				finding.StartLine,
				finding.EndLine,
			)
		}
		resultFindings = append(resultFindings, finding)
	}
	changedRangesByFile := buildChangedRangesByFile(changedFiles, r.logger)
	filteredFindings := splitFindingsByChangedRanges(resultFindings, changedRangesByFile, r.logger)

	r.logger.Debugf("The LLM review produced %d findings after changed-line alignment.", len(filteredFindings))

	return usecase.LLMReviewResult{
		Summary:  decoded.Summary,
		Findings: filteredFindings,
	}, nil
}
