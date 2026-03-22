package llm

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/bentos-lab/peer/domain"
	"github.com/bentos-lab/peer/usecase"
)

type lineRange struct {
	Start int
	End   int
	Side  domain.LineSideEnum
}

var unifiedDiffHunkPattern = regexp.MustCompile(`^@@ -(\d+)(?:,\d+)? \+(\d+)(?:,(\d+))? @@`)

func buildChangedRangesByFile(changedFiles []domain.ChangedFile, logger usecase.Logger) map[string][]lineRange {
	rangesByFile := make(map[string][]lineRange, len(changedFiles))

	for _, file := range changedFiles {
		path := strings.TrimSpace(file.Path)
		if path == "" {
			continue
		}

		ranges, err := parseChangedRangesFromDiff(file.DiffSnippet)
		if err != nil {
			logger.Warnf("Skipping changed-line alignment metadata for file %q because diff parsing failed: %v.", path, err)
			continue
		}
		if len(ranges) == 0 {
			continue
		}
		rangesByFile[path] = ranges
	}

	return rangesByFile
}

func parseChangedRangesFromDiff(diffSnippet string) ([]lineRange, error) {
	diffSnippet = strings.TrimSpace(diffSnippet)
	if diffSnippet == "" {
		return nil, nil
	}

	lines := strings.Split(diffSnippet, "\n")
	newLines := make([]int, 0)
	oldLines := make([]int, 0)
	inHunk := false
	currentNewLine := 0
	currentOldLine := 0
	seenHunk := false

	for _, rawLine := range lines {
		line := strings.TrimSuffix(rawLine, "\r")
		if matches := unifiedDiffHunkPattern.FindStringSubmatch(line); matches != nil {
			oldStart, err := strconv.Atoi(matches[1])
			if err != nil {
				return nil, fmt.Errorf("invalid hunk old start %q", matches[1])
			}
			newStart, err := strconv.Atoi(matches[2])
			if err != nil {
				return nil, fmt.Errorf("invalid hunk new start %q", matches[2])
			}

			currentOldLine = oldStart
			currentNewLine = newStart
			inHunk = true
			seenHunk = true
			continue
		}

		if !inHunk {
			continue
		}

		if strings.HasPrefix(line, "diff --git ") {
			inHunk = false
			continue
		}
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			newLines = append(newLines, currentNewLine)
			currentNewLine++
			continue
		}
		if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			oldLines = append(oldLines, currentOldLine)
			currentOldLine++
			continue
		}
		if strings.HasPrefix(line, " ") {
			currentOldLine++
			currentNewLine++
			continue
		}
		if line == `\ No newline at end of file` {
			continue
		}
		if strings.HasPrefix(line, "--- ") || strings.HasPrefix(line, "+++ ") {
			inHunk = false
			continue
		}
	}

	if !seenHunk {
		return nil, fmt.Errorf("no unified diff hunk found")
	}

	merged := mergeLineNumbersToRanges(newLines, domain.LineSideNew)
	merged = append(merged, mergeLineNumbersToRanges(oldLines, domain.LineSideOld)...)
	sort.Slice(merged, func(i, j int) bool {
		if merged[i].Start == merged[j].Start {
			return merged[i].Side < merged[j].Side
		}
		return merged[i].Start < merged[j].Start
	})
	return merged, nil
}

func mergeLineNumbersToRanges(lines []int, side domain.LineSideEnum) []lineRange {
	if len(lines) == 0 {
		return nil
	}

	ranges := make([]lineRange, 0, len(lines))
	current := lineRange{Start: lines[0], End: lines[0], Side: side}
	for _, line := range lines[1:] {
		if line <= 0 {
			continue
		}
		if line == current.End+1 {
			current.End = line
			continue
		}
		if line <= current.End {
			continue
		}
		ranges = append(ranges, current)
		current = lineRange{Start: line, End: line, Side: side}
	}
	ranges = append(ranges, current)
	return ranges
}
