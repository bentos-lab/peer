package codeagent

import (
	"context"
	"fmt"
	"strings"

	"github.com/bentos-lab/peer/domain"
	"github.com/bentos-lab/peer/shared/refs"
	sharedtext "github.com/bentos-lab/peer/shared/text"
	"github.com/bentos-lab/peer/usecase"
	"github.com/bentos-lab/peer/usecase/contracts"
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

type issueAlignmentKeyIdeasTemplateData struct {
	Issues []issueAlignmentIssueData
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

	base, head := refs.NormalizePromptRefs(payload.Input.Base, payload.Input.Head)

	data := issueAlignmentTaskTemplateData{
		Repository:    payload.Input.Target.Repository,
		Base:          base,
		Head:          head,
		Title:         payload.Input.Title,
		Description:   sharedtext.SingleLine(payload.Input.Description),
		KeyIdeas:      keyIdeas,
		Issues:        mapIssueCandidates(payload.IssueAlignment.Candidates),
		Files:         files,
		ExtraGuidance: strings.TrimSpace(payload.ExtraGuidance),
	}

	return sharedtext.RenderSimpleTemplate("issue_alignment_task", taskTemplate, data)
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
			Body:       sharedtext.SingleLine(issue.Body),
			Comments:   mapIssueComments(candidate.Comments),
		})
	}
	return mapped
}

func mapIssueComments(comments []domain.Comment) []domain.Comment {
	if len(comments) == 0 {
		return nil
	}
	mapped := make([]domain.Comment, 0, len(comments))
	for _, comment := range comments {
		if strings.TrimSpace(comment.Body) == "" {
			continue
		}
		mapped = append(mapped, domain.Comment{
			Author: comment.Author,
			Body:   sharedtext.SingleLine(comment.Body),
		})
	}
	return mapped
}

func renderIssueKeyIdeasPrompt(candidates []domain.IssueContext, promptTemplate string) (string, error) {
	data := issueAlignmentKeyIdeasTemplateData{
		Issues: mapIssueCandidates(candidates),
	}
	return sharedtext.RenderSimpleTemplate("issue_alignment_key_ideas", promptTemplate, data)
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
