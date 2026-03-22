package refs

import (
	"fmt"
	"strings"
)

// ValidateResolvedRefs trims and validates base/head refs for coding-agent prompts.
func ValidateResolvedRefs(base string, head string) (string, string, error) {
	base = strings.TrimSpace(base)
	head = strings.TrimSpace(head)
	if base == "" {
		return "", "", fmt.Errorf("base ref is required")
	}
	if head == "" {
		return "", "", fmt.Errorf("head ref is required")
	}
	if strings.HasPrefix(base, "@") {
		return "", "", fmt.Errorf("base ref must not use workspace tokens")
	}
	if strings.HasPrefix(head, "@") {
		return "", "", fmt.Errorf("head ref must not use workspace tokens")
	}
	return base, head, nil
}
