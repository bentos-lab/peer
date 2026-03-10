package text

import (
	"regexp"
	"strings"
)

// ContainsTrigger reports whether the text includes @name or /name.
func ContainsTrigger(input string, name string) bool {
	pattern := triggerPattern(name)
	if pattern == nil {
		return false
	}
	return pattern.MatchString(input)
}

// StripTrigger removes all @name and /name occurrences and normalizes whitespace.
func StripTrigger(input string, name string) string {
	pattern := triggerPattern(name)
	if pattern == nil {
		return strings.TrimSpace(input)
	}
	cleaned := pattern.ReplaceAllString(input, "")
	lines := strings.Split(cleaned, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(line)
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func triggerPattern(name string) *regexp.Regexp {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return nil
	}
	escaped := regexp.QuoteMeta(trimmed)
	return regexp.MustCompile(`(?i)(?:@|/)` + escaped + `\b`)
}
