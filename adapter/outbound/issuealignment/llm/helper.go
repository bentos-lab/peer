package llm

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"bentos-backend/domain"
	sharedtext "bentos-backend/shared/text"
	"bentos-backend/usecase"
)

type issueAlignmentTaskTemplateData struct {
	Repository    string
	Base          string
	Head          string
	Title         string
	Description   string
	KeyIdeas      []string
	Issues        []issueAlignmentIssueData
	Files         []issueAlignmentFileData
	ExtraGuidance string
}

type issueAlignmentIssueData struct {
	Repository string
	Number     int
	Title      string
	Body       string
	Comments   []domain.Comment
}

type issueAlignmentFileData struct {
	Path        string
	ChangedText string
}

const issueAlignmentSystemPrompt = "You are a senior engineer evaluating issue alignment."

const issueKeyIdeasSystemPrompt = "Extract the main, true requirements from the issue contents. Return only what is explicitly supported."

func issueAlignmentResponseSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"issue", "keyIdeas", "requirements"},
		"properties": map[string]any{
			"issue": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []string{"number"},
				"properties": map[string]any{
					"repository": map[string]any{"type": "string"},
					"number":     map[string]any{"type": "integer"},
					"title":      map[string]any{"type": "string"},
				},
			},
			"keyIdeas": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
			"requirements": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required":             []string{"requirement", "coverage"},
					"properties": map[string]any{
						"requirement": map[string]any{"type": "string"},
						"coverage":    map[string]any{"type": "string"},
					},
				},
			},
		},
	}
}

func issueKeyIdeasSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"keyIdeas"},
		"properties": map[string]any{
			"keyIdeas": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
		},
	}
}

func renderIssueAlignmentTask(payload usecase.LLMIssueAlignmentPayload, keyIdeas []string, changedFiles []domain.ChangedFile, taskTemplate string) (string, error) {
	files := make([]issueAlignmentFileData, 0, len(changedFiles))
	for _, file := range changedFiles {
		changedText := file.DiffSnippet
		if changedText == "" {
			changedText = file.Content
		}
		if strings.TrimSpace(changedText) == "" {
			continue
		}
		files = append(files, issueAlignmentFileData{Path: file.Path, ChangedText: changedText})
	}

	data := issueAlignmentTaskTemplateData{
		Repository:    payload.Input.Target.Repository,
		Base:          payload.Input.Base,
		Head:          payload.Input.Head,
		Title:         payload.Input.Title,
		Description:   sharedtext.SingleLine(payload.Input.Description),
		KeyIdeas:      keyIdeas,
		Issues:        mapIssueCandidates(payload.IssueAlignment.Candidates),
		Files:         files,
		ExtraGuidance: strings.TrimSpace(payload.ExtraGuidance),
	}

	parsedTemplate, err := template.New("issue_alignment_task").Parse(taskTemplate)
	if err != nil {
		return "", err
	}

	var rendered bytes.Buffer
	if err := parsedTemplate.Execute(&rendered, data); err != nil {
		return "", err
	}

	return rendered.String(), nil
}

func mapIssueCandidates(candidates []domain.IssueContext) []issueAlignmentIssueData {
	if len(candidates) == 0 {
		return nil
	}
	mapped := make([]issueAlignmentIssueData, 0, len(candidates))
	for _, candidate := range candidates {
		issue := candidate.Issue
		mapped = append(mapped, issueAlignmentIssueData{
			Repository: issue.Repository,
			Number:     issue.Number,
			Title:      issue.Title,
			Body:       issue.Body,
			Comments:   candidate.Comments,
		})
	}
	return mapped
}

func renderIssueKeyIdeasPrompt(candidates []domain.IssueContext) (string, error) {
	var builder strings.Builder
	builder.WriteString("Extract the main, true requirements from the issue contents below.\n")
	builder.WriteString("- Merge duplicates and keep only distinct requirements.\n")
	builder.WriteString("- Prefer explicit requirements stated in the issue text.\n")
	builder.WriteString("- Keep the list concise.\n\n")
	builder.WriteString("Issues:\n")
	for _, candidate := range candidates {
		builder.WriteString(fmt.Sprintf("- %s#%d: %s\n", candidate.Issue.Repository, candidate.Issue.Number, candidate.Issue.Title))
		if strings.TrimSpace(candidate.Issue.Body) != "" {
			builder.WriteString(fmt.Sprintf("  Body: %s\n", sharedtext.SingleLine(candidate.Issue.Body)))
		}
		if len(candidate.Comments) > 0 {
			builder.WriteString("  Comments:\n")
			for _, comment := range candidate.Comments {
				if strings.TrimSpace(comment.Body) == "" {
					continue
				}
				builder.WriteString(fmt.Sprintf("  - %s: %s\n", comment.Author.Login, sharedtext.SingleLine(comment.Body)))
			}
		}
	}
	return builder.String(), nil
}

func normalizeKeyIdeas(ideas []string) []string {
	if len(ideas) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(ideas))
	filtered := make([]string, 0, len(ideas))
	for _, idea := range ideas {
		trimmed := strings.TrimSpace(idea)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		filtered = append(filtered, trimmed)
	}
	return filtered
}

func fallbackIssueReference(candidates []domain.IssueContext) domain.IssueReference {
	if len(candidates) == 0 {
		return domain.IssueReference{}
	}
	issue := candidates[0].Issue
	return domain.IssueReference{
		Repository: issue.Repository,
		Number:     issue.Number,
		Title:      issue.Title,
	}
}
