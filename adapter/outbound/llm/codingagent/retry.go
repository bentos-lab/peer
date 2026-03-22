package codingagent

import (
	"fmt"
	"strings"
)

func buildJSONFixMessage(validationErr error, lastOutput string) string {
	message := fmt.Sprintf("Your previous response was invalid JSON or did not match the schema: %v.", validationErr)
	if strings.TrimSpace(lastOutput) != "" {
		message = fmt.Sprintf("%s Invalid output was: %s", message, truncateRetryOutput(lastOutput, 500))
	}
	return message + " Please fix the output and return JSON only that strictly matches the schema."
}

func truncateRetryOutput(output string, maxChars int) string {
	if maxChars <= 0 || len(output) <= maxChars {
		return output
	}
	return output[:maxChars] + "...(truncated)"
}
