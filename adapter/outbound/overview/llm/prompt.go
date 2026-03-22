package llm

import (
	"github.com/bentos-lab/peer/domain"
	sharedtext "github.com/bentos-lab/peer/shared/text"
)

func renderOverviewSystemPrompt() (string, error) {
	return sharedtext.RenderSimpleTemplate("overview_system_prompt", overviewSystemPromptTemplateRaw, nil)
}

func renderOverviewUserPrompt(input domain.ChangeRequestInput, changedFiles []domain.ChangedFile) (string, error) {
	files := make([]overviewUserPromptFileData, 0, len(changedFiles))
	for _, file := range changedFiles {
		changedText := file.DiffSnippet
		if changedText == "" {
			changedText = file.Content
		}
		if changedText == "" {
			continue
		}
		files = append(files, overviewUserPromptFileData{
			Path:        file.Path,
			ChangedText: changedText,
		})
	}

	return sharedtext.RenderSimpleTemplate("overview_user_prompt", overviewUserPromptTemplateRaw, overviewUserPromptTemplateData{
		Title:       input.Title,
		Description: sharedtext.SingleLine(input.Description),
		Files:       files,
	})
}
