package llm

import (
	"strings"

	"github.com/bentos-lab/peer/domain"
	"github.com/bentos-lab/peer/usecase"
)

func splitFindingsByChangedRanges(findings []domain.Finding, changedRangesByFile map[string][]lineRange, logger usecase.Logger) []domain.Finding {
	filtered := make([]domain.Finding, 0, len(findings))

	for _, finding := range findings {
		path := strings.TrimSpace(finding.FilePath)
		changedRanges := changedRangesByFile[path]
		lineSide := finding.LineSide
		var segments []lineRange
		if strings.TrimSpace(string(lineSide)) != "" {
			segments = intersectRangeWithChangedLines(finding.StartLine, finding.EndLine, lineSide, changedRanges)
		} else {
			newSegments := intersectRangeWithChangedLines(finding.StartLine, finding.EndLine, domain.LineSideNew, changedRanges)
			oldSegments := intersectRangeWithChangedLines(finding.StartLine, finding.EndLine, domain.LineSideOld, changedRanges)
			newOverlap := overlapLength(newSegments)
			oldOverlap := overlapLength(oldSegments)
			if oldOverlap > newOverlap {
				lineSide = domain.LineSideOld
				segments = oldSegments
			} else {
				lineSide = domain.LineSideNew
				segments = newSegments
			}
		}
		if len(segments) == 0 {
			logger.Warnf("Dropping finding because its range does not overlap changed lines file=%q startLine=%d endLine=%d title=%q.", finding.FilePath, finding.StartLine, finding.EndLine, finding.Title)
			continue
		}

		droppedSegments := 0
		if len(segments) > maxSplitFindingsPerOriginal {
			droppedSegments = len(segments) - maxSplitFindingsPerOriginal
			segments = segments[:maxSplitFindingsPerOriginal]
		}

		logger.Debugf("Aligned finding to changed lines file=%q startLine=%d endLine=%d splitCount=%d title=%q.",
			finding.FilePath, finding.StartLine, finding.EndLine, len(segments), finding.Title)
		if droppedSegments > 0 {
			logger.Warnf("Dropped split finding segments because split count exceeded limit file=%q startLine=%d endLine=%d kept=%d dropped=%d title=%q.",
				finding.FilePath, finding.StartLine, finding.EndLine, len(segments), droppedSegments, finding.Title)
		}

		for _, segment := range segments {
			derived := finding
			derived.StartLine = segment.Start
			derived.EndLine = segment.End
			derived.LineSide = lineSide
			filtered = append(filtered, derived)
		}
	}

	return filtered
}

func intersectRangeWithChangedLines(startLine int, endLine int, side domain.LineSideEnum, changedRanges []lineRange) []lineRange {
	if startLine <= 0 || endLine <= 0 || startLine > endLine || len(changedRanges) == 0 {
		return nil
	}

	result := make([]lineRange, 0)
	for _, changed := range changedRanges {
		if changed.Side != side {
			continue
		}
		if changed.End < startLine {
			continue
		}
		if changed.Start > endLine {
			break
		}
		overlapStart := startLine
		if changed.Start > overlapStart {
			overlapStart = changed.Start
		}
		overlapEnd := endLine
		if changed.End < overlapEnd {
			overlapEnd = changed.End
		}
		if overlapStart <= overlapEnd {
			result = append(result, lineRange{Start: overlapStart, End: overlapEnd, Side: side})
		}
	}
	return result
}

func overlapLength(ranges []lineRange) int {
	total := 0
	for _, item := range ranges {
		if item.End < item.Start {
			continue
		}
		total += item.End - item.Start + 1
	}
	return total
}
