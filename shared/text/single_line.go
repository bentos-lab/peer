package text

import "strings"

// SingleLine collapses all whitespace into single spaces and trims the result.
func SingleLine(input string) string {
	fields := strings.Fields(input)
	if len(fields) == 0 {
		return ""
	}
	return strings.Join(fields, " ")
}
