package safetysanitizer

import (
	sharedtext "github.com/bentos-lab/peer/shared/text"
)

func renderSystemPrompt(options Options) (string, error) {
	return sharedtext.RenderSimpleTemplate("safety_sanitizer_system_prompt", sanitizerSystemPromptTemplateRaw, options)
}
