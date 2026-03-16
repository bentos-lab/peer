package refs

import "strings"

// NormalizePromptRefs trims base/head refs for coding-agent prompts without dropping base.
func NormalizePromptRefs(base string, head string) (string, string) {
	return strings.TrimSpace(base), strings.TrimSpace(head)
}
