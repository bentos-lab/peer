package llm

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"
	"time"

	"bentos-backend/domain"
	"bentos-backend/shared/logger/stdlogger"
	sharedtext "bentos-backend/shared/text"
	"bentos-backend/usecase"
	"bentos-backend/usecase/contracts"
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
	startedAt := time.Now()
	r.logger.Infof("LLM review started.")
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
		r.logger.Errorf("LLM review failed while rendering the system prompt.")
		r.logger.Debugf("The LLM review ran for %d ms before failing.", time.Since(startedAt).Milliseconds())
		r.logger.Debugf("Failure details: %v.", err)
		return usecase.LLMReviewResult{}, err
	}

	userPrompt, err := renderUserPrompt(payload.Input, changedFiles)
	if err != nil {
		r.logger.Errorf("LLM review failed while rendering the user prompt.")
		r.logger.Debugf("The LLM review ran for %d ms before failing.", time.Since(startedAt).Milliseconds())
		r.logger.Debugf("Failure details: %v.", err)
		return usecase.LLMReviewResult{}, err
	}

	outputMap, err := r.generator.GenerateJSON(ctx, contracts.GenerateParams{
		SystemPrompt: systemPrompt,
		Messages:     []string{userPrompt},
	}, reviewResponseSchema())
	if err != nil {
		r.logger.Errorf("LLM review failed while requesting JSON output.")
		r.logger.Debugf("The LLM review ran for %d ms before failing.", time.Since(startedAt).Milliseconds())
		r.logger.Debugf("Failure details: %v.", err)
		return usecase.LLMReviewResult{}, err
	}

	raw, err := json.Marshal(outputMap)
	if err != nil {
		r.logger.Errorf("LLM review failed while encoding model output.")
		r.logger.Debugf("The LLM review ran for %d ms before failing.", time.Since(startedAt).Milliseconds())
		r.logger.Debugf("Failure details: %v.", err)
		return usecase.LLMReviewResult{}, err
	}

	var decoded reviewModelOutput
	if err := json.Unmarshal(raw, &decoded); err != nil {
		err = fmt.Errorf("invalid review model output: %w", err)
		r.logger.Errorf("LLM review failed because the model output is invalid.")
		r.logger.Debugf("The LLM review ran for %d ms before failing.", time.Since(startedAt).Milliseconds())
		r.logger.Debugf("Failure details: %v.", err)
		return usecase.LLMReviewResult{}, err
	}

	findings := make([]json.RawMessage, 0)
	if decoded.Findings != nil {
		findings = decoded.Findings
	}

	resultFindings := make([]domain.Finding, 0, len(findings))
	for _, findingRaw := range findings {
		var finding domain.Finding
		if err := json.Unmarshal(findingRaw, &finding); err != nil {
			err = fmt.Errorf("invalid finding format: %w", err)
			r.logger.Errorf("LLM review failed because one finding has an invalid format.")
			r.logger.Debugf("The LLM review ran for %d ms before failing.", time.Since(startedAt).Milliseconds())
			r.logger.Debugf("Failure details: %v.", err)
			return usecase.LLMReviewResult{}, err
		}
		if finding.StartLine <= 0 || finding.EndLine <= 0 || finding.StartLine > finding.EndLine {
			err = fmt.Errorf("invalid finding range for %q: start line is %d and end line is %d", finding.FilePath, finding.StartLine, finding.EndLine)
			r.logger.Errorf("LLM review failed because one finding has an invalid range.")
			r.logger.Debugf("The LLM review ran for %d ms before failing.", time.Since(startedAt).Milliseconds())
			r.logger.Debugf("Failure details: %v.", err)
			return usecase.LLMReviewResult{}, err
		}
		resultFindings = append(resultFindings, finding)
	}
	changedRangesByFile := buildChangedRangesByFile(changedFiles, r.logger)
	filteredFindings := splitFindingsByChangedRanges(resultFindings, changedRangesByFile, r.logger)

	r.logger.Infof("LLM review completed.")
	r.logger.Debugf("The LLM review completed in %d ms.", time.Since(startedAt).Milliseconds())
	r.logger.Debugf("The LLM review produced %d findings after changed-line alignment.", len(filteredFindings))

	return usecase.LLMReviewResult{
		Summary:  decoded.Summary,
		Findings: filteredFindings,
	}, nil
}

func reviewResponseSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"summary", "findings"},
		"properties": map[string]any{
			"summary": map[string]any{
				"type": "string",
			},
			"findings": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required": []string{
						"filePath",
						"startLine",
						"endLine",
						"severity",
						"title",
						"detail",
						"suggestion",
					},
					"properties": map[string]any{
						"filePath": map[string]any{
							"type": "string",
						},
						"startLine": map[string]any{
							"type":    "integer",
							"minimum": 1,
						},
						"endLine": map[string]any{
							"type":    "integer",
							"minimum": 1,
						},
						"severity": map[string]any{
							"type": "string",
							"enum": []string{
								string(domain.FindingSeverityCritical),
								string(domain.FindingSeverityMajor),
								string(domain.FindingSeverityMinor),
								string(domain.FindingSeverityNit),
							},
						},
						"title": map[string]any{
							"type": "string",
						},
						"detail": map[string]any{
							"type": "string",
						},
						"suggestion": map[string]any{
							"type": "string",
						},
					},
				},
			},
		},
	}
}

func renderSystemPrompt(rulePackText string) (string, error) {
	parsedTemplate, err := template.New("reviewer_system_prompt").Parse(reviewSystemPromptTemplateRaw)
	if err != nil {
		return "", err
	}

	var rendered bytes.Buffer
	if err := parsedTemplate.Execute(&rendered, reviewSystemPromptTemplateData{
		RulePackText: rulePackText,
	}); err != nil {
		return "", err
	}

	return rendered.String(), nil
}

func renderUserPrompt(input domain.ChangeRequestInput, changedFiles []domain.ChangedFile) (string, error) {
	files := make([]reviewUserPromptFileData, 0, len(changedFiles))
	for _, file := range changedFiles {
		changedText := file.DiffSnippet
		if changedText == "" {
			changedText = file.Content
		}
		if changedText == "" {
			continue
		}
		files = append(files, reviewUserPromptFileData{
			Path:        file.Path,
			ChangedText: changedText,
		})
	}

	language := input.Language
	if language == "" {
		language = "English"
	}

	parsedTemplate, err := template.New("reviewer_user_prompt").Parse(reviewUserPromptTemplateRaw)
	if err != nil {
		return "", err
	}

	var rendered bytes.Buffer
	if err := parsedTemplate.Execute(&rendered, reviewUserPromptTemplateData{
		Title:       input.Title,
		Description: sharedtext.SingleLine(input.Description),
		Language:    language,
		Files:       files,
	}); err != nil {
		return "", err
	}

	return rendered.String(), nil
}
