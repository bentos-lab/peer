package llm

import (
	"testing"

	"bentos-backend/domain"
	"bentos-backend/shared/logger/stdlogger"
	"github.com/stretchr/testify/require"
)

func TestParseChangedRangesFromDiffTracksBothSides(t *testing.T) {
	diff := "@@ -1,2 +1,2 @@\n-line1\n+line1 updated\n line2\n"
	ranges, err := parseChangedRangesFromDiff(diff)
	require.NoError(t, err)

	require.Contains(t, ranges, lineRange{Start: 1, End: 1, Side: domain.LineSideOld})
	require.Contains(t, ranges, lineRange{Start: 1, End: 1, Side: domain.LineSideNew})
}

func TestSplitFindingsByChangedRangesInfersOldSideWhenOnlyDeletions(t *testing.T) {
	diff := "@@ -3,1 +3,0 @@\n-line3\n"
	ranges, err := parseChangedRangesFromDiff(diff)
	require.NoError(t, err)

	changedRangesByFile := map[string][]lineRange{
		"main.go": ranges,
	}
	findings := []domain.Finding{
		{FilePath: "main.go", StartLine: 3, EndLine: 3},
	}

	filtered := splitFindingsByChangedRanges(findings, changedRangesByFile, stdlogger.Nop())
	require.Len(t, filtered, 1)
	require.Equal(t, domain.LineSideOld, filtered[0].LineSide)
	require.Equal(t, 3, filtered[0].StartLine)
	require.Equal(t, 3, filtered[0].EndLine)
}

func TestSplitFindingsByChangedRangesPrefersSideWithLargerOverlap(t *testing.T) {
	diff := "@@ -1,21 +1,1 @@\n-line1\n-line2\n-line3\n-line4\n-line5\n-line6\n-line7\n-line8\n-line9\n-line10\n-line11\n-line12\n-line13\n-line14\n-line15\n-line16\n-line17\n-line18\n-line19\n-line20\n-line21\n+single\n"
	ranges, err := parseChangedRangesFromDiff(diff)
	require.NoError(t, err)

	changedRangesByFile := map[string][]lineRange{
		"LICENSE": ranges,
	}
	findings := []domain.Finding{
		{FilePath: "LICENSE", StartLine: 1, EndLine: 21},
	}

	filtered := splitFindingsByChangedRanges(findings, changedRangesByFile, stdlogger.Nop())
	require.Len(t, filtered, 1)
	require.Equal(t, domain.LineSideOld, filtered[0].LineSide)
	require.Equal(t, 1, filtered[0].StartLine)
	require.Equal(t, 21, filtered[0].EndLine)
}
