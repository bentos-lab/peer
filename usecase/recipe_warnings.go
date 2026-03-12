package usecase

import (
	"sort"
	"strings"
)

// FormatRecipeWarning formats missing recipe paths as a GitHub markdown warning block.
func FormatRecipeWarning(paths []string) string {
	normalized := normalizeRecipeWarnings(paths)
	if len(normalized) == 0 {
		return ""
	}

	var builder strings.Builder
	builder.WriteString("> [!WARNING]\n")
	builder.WriteString("> Missing custom recipe file(s):\n")
	for _, path := range normalized {
		builder.WriteString("> - ")
		builder.WriteString(path)
		builder.WriteString("\n")
	}
	return strings.TrimRight(builder.String(), "\n")
}

func normalizeRecipeWarnings(paths []string) []string {
	unique := map[string]struct{}{}
	for _, path := range paths {
		trimmed := strings.TrimSpace(path)
		if trimmed == "" {
			continue
		}
		unique[trimmed] = struct{}{}
	}
	if len(unique) == 0 {
		return nil
	}
	list := make([]string, 0, len(unique))
	for path := range unique {
		list = append(list, path)
	}
	sort.Strings(list)
	return list
}
