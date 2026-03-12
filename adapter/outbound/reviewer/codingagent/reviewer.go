package codingagent

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

//go:embed task.md
var reviewTaskPromptTemplateRaw string

//go:embed formatting_system.md
var reviewFormattingSystemPrompt string

// Config contains coding-agent runtime options.
type Config struct {
	Agent    string
	Provider string
	Model    string
}

// Reviewer uses a coding agent for analysis and an LLM for JSON formatting.
type Reviewer struct {
	formatter contracts.LLMGenerator
	config    Config
	logger    usecase.Logger
}

type reviewModelOutput struct {
	Summary  string            `json:"summary"`
	Findings []json.RawMessage `json:"findings"`
}

type reviewTaskPromptTemplateData struct {
	Repository    string
	RepoURL       string
	Base          string
	Head          string
	Title         string
	Description   string
	Language      string
	Suggestions   bool
	RulePackText  string
	CustomRuleset string
}

// NewReviewer creates a coding-agent reviewer adapter.
func NewReviewer(formatter contracts.LLMGenerator, config Config, logger usecase.Logger) (*Reviewer, error) {
	if formatter == nil {
		return nil, fmt.Errorf("formatter llm generator must not be nil")
	}
	if strings.TrimSpace(config.Agent) == "" {
		return nil, fmt.Errorf("coding agent is required")
	}
	if logger == nil {
		logger = stdlogger.Nop()
	}
	return &Reviewer{
		formatter: formatter,
		config:    config,
		logger:    logger,
	}, nil
}

// Review generates findings using coding-agent analysis and formatter conversion.
func (r *Reviewer) Review(ctx context.Context, payload usecase.LLMReviewPayload) (usecase.LLMReviewResult, error) {
	startedAt := time.Now()
	r.logger.Infof("Coding-agent review started.")

	normalizedBase, normalizedHead := normalizePromptRefs(payload.Input.Base, payload.Input.Head)

	if payload.Environment == nil {
		return usecase.LLMReviewResult{}, fmt.Errorf("code environment must not be nil")
	}
	if err := ensureDiffContentAvailable(ctx, payload.Environment, normalizedBase, normalizedHead); err != nil {
		return usecase.LLMReviewResult{}, err
	}

	agent, err := r.setupAgent(ctx, payload.Environment, normalizedHead)
	if err != nil {
		return usecase.LLMReviewResult{}, err
	}
	taskPrompt, err := renderSimpleTemplate("review_task_prompt", reviewTaskPromptTemplateRaw, reviewTaskPromptTemplateData{
		Repository:    payload.Input.Target.Repository,
		RepoURL:       payload.Input.RepoURL,
		Base:          normalizedBase,
		Head:          normalizedHead,
		Title:         payload.Input.Title,
		Description:   sharedtext.SingleLine(payload.Input.Description),
		Language:      resolveLanguage(payload.Input.Language),
		Suggestions:   payload.Suggestions,
		RulePackText:  strings.Join(payload.RulePack.Instructions, "\n\n"),
		CustomRuleset: strings.TrimSpace(payload.CustomRuleset),
	})
	if err != nil {
		return usecase.LLMReviewResult{}, err
	}

	rawText, err := runTask(ctx, agent, r.config, taskPrompt)
	if err != nil {
		return usecase.LLMReviewResult{}, err
	}

	outputMap, err := r.formatter.GenerateJSON(ctx, contracts.GenerateParams{
		SystemPrompt: reviewFormattingSystemPrompt,
		Messages:     []string{rawText},
	}, reviewResponseSchema())
	if err != nil {
		return usecase.LLMReviewResult{}, err
	}

	var decoded reviewModelOutput
	if err := decodeModelOutput(outputMap, &decoded); err != nil {
		return usecase.LLMReviewResult{}, err
	}

	findings := make([]domain.Finding, 0, len(decoded.Findings))
	for _, findingRaw := range decoded.Findings {
		var finding domain.Finding
		if err := json.Unmarshal(findingRaw, &finding); err != nil {
			return usecase.LLMReviewResult{}, fmt.Errorf("invalid finding format: %w", err)
		}
		if finding.StartLine <= 0 || finding.EndLine <= 0 || finding.StartLine > finding.EndLine {
			return usecase.LLMReviewResult{}, fmt.Errorf("invalid finding range for %q: %d-%d", finding.FilePath, finding.StartLine, finding.EndLine)
		}
		finding.SuggestedChange = normalizeSuggestedChange(finding.SuggestedChange)
		findings = append(findings, finding)
	}

	r.logger.Infof("Coding-agent review completed.")
	r.logger.Debugf("Coding-agent review took %d ms and produced %d findings.", time.Since(startedAt).Milliseconds(), len(findings))
	return usecase.LLMReviewResult{Summary: decoded.Summary, Findings: findings}, nil
}

func normalizeSuggestedChange(change *domain.SuggestedChange) *domain.SuggestedChange {
	if change == nil {
		return nil
	}
	change.Kind = domain.SuggestedChangeKindEnum(strings.TrimSpace(string(change.Kind)))
	change.Reason = strings.TrimSpace(change.Reason)
	if change.StartLine <= 0 || change.EndLine <= 0 || change.StartLine > change.EndLine {
		return nil
	}
	if change.Kind != domain.SuggestedChangeKindReplace && change.Kind != domain.SuggestedChangeKindDelete {
		return nil
	}
	if change.Reason == "" {
		return nil
	}
	if change.Kind == domain.SuggestedChangeKindDelete {
		if strings.TrimSpace(change.Replacement) != "" {
			return nil
		}
		change.Replacement = ""
		return change
	}
	if strings.TrimSpace(change.Replacement) == "" {
		return nil
	}
	return change
}

func resolveLanguage(language string) string {
	trimmed := strings.TrimSpace(language)
	if trimmed == "" {
		return "English"
	}
	return trimmed
}

func normalizePromptRefs(base string, head string) (string, string) {
	normalizedBase := strings.TrimSpace(base)
	normalizedHead := strings.TrimSpace(head)
	if normalizedHead == "@staged" || normalizedHead == "@all" {
		return "", normalizedHead
	}
	return normalizedBase, normalizedHead
}

func ensureDiffContentAvailable(ctx context.Context, environment contracts.CodeEnvironment, base string, head string) error {
	changedFiles, err := environment.LoadChangedFiles(ctx, domain.CodeEnvironmentLoadOptions{
		Base: base,
		Head: head,
	})
	if err != nil {
		return fmt.Errorf("failed to load changed files: %w", err)
	}
	for _, file := range changedFiles {
		if strings.TrimSpace(file.DiffSnippet) != "" {
			return nil
		}
	}
	return fmt.Errorf("diff content is empty for base %q and head %q", base, head)
}

func (r *Reviewer) setupAgent(ctx context.Context, environment contracts.CodeEnvironment, head string) (contracts.CodingAgent, error) {
	agent, err := environment.SetupAgent(ctx, domain.CodingAgentSetupOptions{
		Agent: r.config.Agent,
		Ref:   strings.TrimSpace(head),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to setup coding agent: %w", err)
	}
	return agent, nil
}

func runTask(ctx context.Context, agent contracts.CodingAgent, cfg Config, task string) (string, error) {
	result, err := agent.Run(ctx, strings.TrimSpace(task), domain.CodingAgentRunOptions{
		Provider: cfg.Provider,
		Model:    cfg.Model,
	})
	if err != nil {
		return "", fmt.Errorf("failed to run coding agent task: %w", err)
	}
	return strings.TrimSpace(result.Text), nil
}

func decodeModelOutput(outputMap map[string]any, target any) error {
	raw, err := json.Marshal(outputMap)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("invalid formatted model output: %w", err)
	}
	return nil
}

func renderSimpleTemplate(templateName string, templateRaw string, input any) (string, error) {
	parsedTemplate, err := template.New(templateName).Parse(templateRaw)
	if err != nil {
		return "", err
	}
	var rendered bytes.Buffer
	if err := parsedTemplate.Execute(&rendered, input); err != nil {
		return "", err
	}
	return rendered.String(), nil
}

func reviewResponseSchema() map[string]any {
	suggestedChangeSchema := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"startLine", "endLine", "kind", "replacement", "reason"},
		"properties": map[string]any{
			"startLine":   map[string]any{"type": "integer", "minimum": 1},
			"endLine":     map[string]any{"type": "integer", "minimum": 1},
			"kind":        map[string]any{"type": "string", "enum": []string{string(domain.SuggestedChangeKindReplace), string(domain.SuggestedChangeKindDelete)}},
			"replacement": map[string]any{"type": "string"},
			"reason":      map[string]any{"type": "string"},
		},
	}

	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"summary", "findings"},
		"properties": map[string]any{
			"summary": map[string]any{"type": "string"},
			"findings": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required":             []string{"filePath", "startLine", "endLine", "severity", "title", "detail", "suggestion"},
					"properties": map[string]any{
						"filePath":        map[string]any{"type": "string"},
						"startLine":       map[string]any{"type": "integer", "minimum": 1},
						"endLine":         map[string]any{"type": "integer", "minimum": 1},
						"severity":        map[string]any{"type": "string", "enum": []string{string(domain.FindingSeverityCritical), string(domain.FindingSeverityMajor), string(domain.FindingSeverityMinor), string(domain.FindingSeverityNit)}},
						"title":           map[string]any{"type": "string"},
						"detail":          map[string]any{"type": "string"},
						"suggestion":      map[string]any{"type": "string"},
						"suggestedChange": map[string]any{"anyOf": []any{suggestedChangeSchema, map[string]any{"type": "null"}}},
					},
				},
			},
		},
	}
}
