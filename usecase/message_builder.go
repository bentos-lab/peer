package usecase

import (
	"fmt"
	"sort"
	"strings"

	"github.com/bentos-lab/peer/domain"
)

// BuildMessages groups findings by file and appends one short summary.
func BuildMessages(findings []domain.Finding, llmSummary string) []domain.ReviewMessage {
	grouped := map[string][]domain.Finding{}
	for _, finding := range findings {
		path := finding.FilePath
		if path == "" {
			path = "unknown"
		}
		grouped[path] = append(grouped[path], finding)
	}

	paths := make([]string, 0, len(grouped))
	for path := range grouped {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	messages := make([]domain.ReviewMessage, 0, len(paths)+1)
	for _, path := range paths {
		items := grouped[path]
		sort.Slice(items, func(i, j int) bool {
			if items[i].StartLine == items[j].StartLine {
				return items[i].Title < items[j].Title
			}
			return items[i].StartLine < items[j].StartLine
		})

		var bodyBuilder strings.Builder
		for _, item := range items {
			lineText := ""
			if item.StartLine > 0 {
				if item.EndLine > item.StartLine {
					lineText = fmt.Sprintf(" (lines %d-%d)", item.StartLine, item.EndLine)
				} else {
					lineText = fmt.Sprintf(" (line %d)", item.StartLine)
				}
			}
			bodyBuilder.WriteString(fmt.Sprintf("- [%s]%s %s: %s", item.Severity, lineText, item.Title, item.Detail))
			if item.Suggestion != "" {
				bodyBuilder.WriteString(fmt.Sprintf(" Suggested change: %s", item.Suggestion))
			}
			bodyBuilder.WriteString("\n")
		}

		messages = append(messages, domain.ReviewMessage{
			Type:         domain.ReviewMessageTypeFileGroup,
			Title:        fmt.Sprintf("Review notes for `%s`", path),
			Body:         strings.TrimSpace(bodyBuilder.String()),
			FilePath:     path,
			FindingCount: len(items),
		})
	}

	summary := llmSummary
	if summary == "" {
		summary = defaultSummary(findings)
	}
	messages = append(messages, domain.ReviewMessage{
		Type:         domain.ReviewMessageTypeSummary,
		Title:        "Review summary",
		Body:         summary,
		FindingCount: len(findings),
	})

	return messages
}

func defaultSummary(findings []domain.Finding) string {
	if len(findings) == 0 {
		return "No significant review findings from changed content."
	}

	countBySeverity := map[domain.FindingSeverityEnum]int{}
	for _, finding := range findings {
		countBySeverity[finding.Severity]++
	}

	return fmt.Sprintf(
		"Found %d potential issue(s): critical=%d, major=%d, minor=%d.",
		len(findings),
		countBySeverity[domain.FindingSeverityCritical],
		countBySeverity[domain.FindingSeverityMajor],
		countBySeverity[domain.FindingSeverityMinor],
	)
}
