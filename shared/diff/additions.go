package diff

import (
	"strconv"
	"strings"
)

// AddedBlock represents one contiguous added block in a unified diff.
type AddedBlock struct {
	StartLine int
	EndLine   int
	Content   string
}

// ExtractAddedBlocks parses a unified diff and returns added line blocks.
func ExtractAddedBlocks(diffSnippet string) []AddedBlock {
	lines := strings.Split(diffSnippet, "\n")
	blocks := make([]AddedBlock, 0)
	currentStart := 0
	currentLines := make([]string, 0)
	currentLineNumber := 0

	flush := func() {
		if currentStart == 0 || len(currentLines) == 0 {
			currentStart = 0
			currentLines = currentLines[:0]
			return
		}
		endLine := currentStart + len(currentLines) - 1
		blocks = append(blocks, AddedBlock{
			StartLine: currentStart,
			EndLine:   endLine,
			Content:   strings.Join(currentLines, "\n"),
		})
		currentStart = 0
		currentLines = currentLines[:0]
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		if strings.HasPrefix(line, "@@ ") {
			flush()
			if newStart, ok := parseNewHunkStart(line); ok {
				currentLineNumber = newStart
			}
			continue
		}

		if strings.HasPrefix(line, "diff --git") || strings.HasPrefix(line, "index ") || strings.HasPrefix(line, "--- ") || strings.HasPrefix(line, "+++ ") {
			flush()
			continue
		}

		if strings.HasPrefix(line, "\\ No newline at end of file") {
			continue
		}

		if strings.HasPrefix(line, "+") {
			if currentStart == 0 {
				currentStart = currentLineNumber
			}
			currentLines = append(currentLines, strings.TrimPrefix(line, "+"))
			currentLineNumber++
			continue
		}

		if strings.HasPrefix(line, "-") {
			flush()
			continue
		}

		if strings.HasPrefix(line, " ") {
			flush()
			currentLineNumber++
			continue
		}
	}

	flush()
	return blocks
}

func parseNewHunkStart(line string) (int, bool) {
	start := strings.Index(line, "+")
	if start == -1 {
		return 0, false
	}
	end := start + 1
	for end < len(line) {
		ch := line[end]
		if ch < '0' || ch > '9' {
			break
		}
		end++
	}
	if end == start+1 {
		return 0, false
	}
	value, err := strconv.Atoi(line[start+1 : end])
	if err != nil || value <= 0 {
		return 0, false
	}
	return value, true
}
