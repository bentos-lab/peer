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
	"bentos-backend/usecase"
	"bentos-backend/usecase/contracts"
)

//go:embed system.md
var systemPromptTemplateRaw string

//go:embed input.md
var userPromptTemplateRaw string

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

// NewReviewer creates an outbound reviewer adapter backed by a generic LLM client.
func NewReviewer(generator contracts.LLMGenerator, logger usecase.Logger) (*Reviewer, error) {
	if generator == nil {
		return nil, fmt.Errorf("llm generator must not be nil")
	}
	if logger == nil {
		logger = usecase.NopLogger
	}
	return &Reviewer{generator: generator, logger: logger}, nil
}

// ReviewDiff generates findings from changed content by calling an LLM provider.
func (r *Reviewer) ReviewDiff(ctx context.Context, payload usecase.LLMReviewPayload) (usecase.LLMReviewResult, error) {
	startedAt := time.Now()
	r.logger.Infof("LLM review started.")
	r.logger.Debugf("The review input includes %d changed files.", len(payload.Input.ChangedFiles))
	r.logger.Debugf("The rule pack includes %d instructions.", len(payload.RulePack.Instructions))

	systemPrompt, err := renderSystemPrompt(strings.Join(payload.RulePack.Instructions, "\n\n"))
	if err != nil {
		r.logger.Errorf("LLM review failed while rendering the system prompt.")
		r.logger.Debugf("The LLM review ran for %d ms before failing.", time.Since(startedAt).Milliseconds())
		r.logger.Debugf("Failure details: %v.", err)
		return usecase.LLMReviewResult{}, err
	}

	userPrompt, err := renderUserPrompt(payload.Input)
	if err != nil {
		r.logger.Errorf("LLM review failed while rendering the user prompt.")
		r.logger.Debugf("The LLM review ran for %d ms before failing.", time.Since(startedAt).Milliseconds())
		r.logger.Debugf("Failure details: %v.", err)
		return usecase.LLMReviewResult{}, err
	}

	outputMap, err := r.generator.GenerateJSON(ctx, contracts.GenerateParams{
		SystemPrompt: systemPrompt,
		Messages: []contracts.Message{
			{
				Role:    "user",
				Content: userPrompt,
			},
		},
		ResponseSchema: reviewResponseSchema(),
	})
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

	r.logger.Infof("LLM review completed.")
	r.logger.Debugf("The LLM review completed in %d ms.", time.Since(startedAt).Milliseconds())
	r.logger.Debugf("The LLM review produced %d findings.", len(resultFindings))

	return usecase.LLMReviewResult{
		Summary:  decoded.Summary,
		Findings: resultFindings,
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
	parsedTemplate, err := template.New("reviewer_system_prompt").Parse(systemPromptTemplateRaw)
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

func renderUserPrompt(input domain.ReviewInput) (string, error) {
	files := make([]reviewUserPromptFileData, 0, len(input.ChangedFiles))
	for _, file := range input.ChangedFiles {
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

	parsedTemplate, err := template.New("reviewer_user_prompt").Parse(userPromptTemplateRaw)
	if err != nil {
		return "", err
	}

	var rendered bytes.Buffer
	if err := parsedTemplate.Execute(&rendered, reviewUserPromptTemplateData{
		Title:       input.Title,
		Description: input.Description,
		Language:    language,
		Files:       files,
	}); err != nil {
		return "", err
	}

	return rendered.String(), nil
}
