package llm

import (
	"github.com/bentos-lab/peer/domain"
	sharedtext "github.com/bentos-lab/peer/shared/text"
)

func renderSystemPrompt(rulePackText string) (string, error) {
	return sharedtext.RenderSimpleTemplate("reviewer_system_prompt", reviewSystemPromptTemplateRaw, reviewSystemPromptTemplateData{
		RulePackText: rulePackText,
	})
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

	return sharedtext.RenderSimpleTemplate("reviewer_user_prompt", reviewUserPromptTemplateRaw, reviewUserPromptTemplateData{
		Title:       input.Title,
		Description: sharedtext.SingleLine(input.Description),
		Language:    language,
		Files:       files,
	})
}
