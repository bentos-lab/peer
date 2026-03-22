package safetysanitizer

import (
	"strings"

	"github.com/bentos-lab/peer/domain"
)

func sanitizeStatus(value string) domain.PromptSafetyStatusEnum {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "ok":
		return domain.PromptSafetyStatusOK
	case "unsafe":
		return domain.PromptSafetyStatusUnsafe
	default:
		return domain.PromptSafetyStatusUnsupported
	}
}

func normalizeStatus(value string) domain.PromptSafetyStatusEnum {
	return sanitizeStatus(value)
}
