package domain

// QuestionSafetyStatusEnum represents the safety classification of a question.
type QuestionSafetyStatusEnum string

const (
	// QuestionSafetyStatusOK indicates the question is safe and supported.
	QuestionSafetyStatusOK QuestionSafetyStatusEnum = "OK"
	// QuestionSafetyStatusUnsupported indicates the question is unsupported.
	QuestionSafetyStatusUnsupported QuestionSafetyStatusEnum = "UNSUPPORTED"
	// QuestionSafetyStatusUnsafe indicates the question is unsafe.
	QuestionSafetyStatusUnsafe QuestionSafetyStatusEnum = "UNSAFE"
)
