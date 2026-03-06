package github

import (
	"errors"
	"fmt"
	"strings"
)

// InvalidAnchorError means GitHub rejected the requested file/line anchor.
type InvalidAnchorError struct {
	Message string
	Cause   error
}

// Error returns the invalid-anchor error message.
func (e *InvalidAnchorError) Error() string {
	if e.Cause == nil {
		return e.Message
	}
	return fmt.Sprintf("%s: %v", e.Message, e.Cause)
}

// Unwrap returns the underlying cause.
func (e *InvalidAnchorError) Unwrap() error {
	return e.Cause
}

// IsInvalidAnchorError reports whether err wraps InvalidAnchorError.
func IsInvalidAnchorError(err error) bool {
	var invalidAnchorErr *InvalidAnchorError
	return errors.As(err, &invalidAnchorErr)
}

func isInvalidAnchorErrorText(text string) bool {
	normalized := strings.ToLower(text)
	return strings.Contains(normalized, "line must be part of the diff") ||
		strings.Contains(normalized, "start_line must be part of the diff") ||
		strings.Contains(normalized, "is outside the diff") ||
		strings.Contains(normalized, "is not part of the diff") ||
		strings.Contains(normalized, "pull_request_review_thread.path") ||
		strings.Contains(normalized, "path is missing") ||
		strings.Contains(normalized, "validation failed")
}

func isInvalidAnchorAPIError(err error) bool {
	errorText := strings.ToLower(err.Error())
	if !strings.Contains(errorText, "422") {
		return false
	}
	return isInvalidAnchorErrorText(errorText)
}

func isInvalidAnchorCommandError(err error) bool {
	errorText := strings.ToLower(err.Error())
	if !strings.Contains(errorText, "422") {
		return false
	}
	return isInvalidAnchorErrorText(errorText)
}
