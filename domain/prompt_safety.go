package domain

// PromptSafetyStatusEnum represents the safety classification of a prompt.
type PromptSafetyStatusEnum string

const (
	// PromptSafetyStatusOK indicates the prompt is safe and supported.
	PromptSafetyStatusOK PromptSafetyStatusEnum = "OK"
	// PromptSafetyStatusUnsupported indicates the prompt is unsupported.
	PromptSafetyStatusUnsupported PromptSafetyStatusEnum = "UNSUPPORTED"
	// PromptSafetyStatusUnsafe indicates the prompt is unsafe.
	PromptSafetyStatusUnsafe PromptSafetyStatusEnum = "UNSAFE"
)
