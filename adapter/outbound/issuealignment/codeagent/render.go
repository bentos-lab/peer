package codeagent

import (
	"strings"

	"github.com/bentos-lab/peer/domain"
	"github.com/bentos-lab/peer/shared/refs"
	sharedtext "github.com/bentos-lab/peer/shared/text"
	"github.com/bentos-lab/peer/usecase"
)

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

func renderIssueKeyIdeasPrompt(candidates []domain.IssueContext, promptTemplate string) (string, error) {
	data := issueAlignmentKeyIdeasTemplateData{
		Issues: mapIssueCandidates(candidates),
	}
	return sharedtext.RenderSimpleTemplate("issue_alignment_key_ideas", promptTemplate, data)
}
