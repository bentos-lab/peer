package gitlab

import (
	"crypto/sha1"
	"encoding/hex"
	"strconv"
	"strings"

	"github.com/bentos-lab/peer/domain"
)

func buildPositionFields(input domain.ReviewCommentInput, info domain.ChangeRequestInfo) ([]field, bool) {
	side := ensureLineSide(input.LineSide)
	fields := []field{
		{name: "position[position_type]", value: "text", raw: true},
		{name: "position[base_sha]", value: info.BaseRef, raw: true},
		{name: "position[start_sha]", value: info.StartRef, raw: true},
		{name: "position[head_sha]", value: info.HeadRef, raw: true},
		{name: "position[new_path]", value: input.Path, raw: true},
		{name: "position[old_path]", value: input.Path, raw: true},
	}

	if input.StartLine != input.EndLine {
		startOld, startNew := lineNumbersForSide(side, input.StartLine)
		endOld, endNew := lineNumbersForSide(side, input.EndLine)
		lineType := lineTypeForSide(side)
		fields = append(fields,
			lineRangeFields("start", lineType, startOld, startNew, input.Path)...,
		)
		fields = append(fields,
			lineRangeFields("end", lineType, endOld, endNew, input.Path)...,
		)
		return fields, true
	}

	oldLine, newLine := lineNumbersForSide(side, input.EndLine)
	if side == domain.LineSideOld {
		fields = append(fields, field{name: "position[old_line]", value: strconv.Itoa(oldLine), raw: true})
	} else {
		fields = append(fields, field{name: "position[new_line]", value: strconv.Itoa(newLine), raw: true})
	}
	return fields, false
}

func lineRangeFields(anchor string, lineType string, oldLine int, newLine int, path string) []field {
	lineCode := buildLineCode(path, oldLine, newLine)
	fields := []field{
		{name: "position[line_range][" + anchor + "][type]", value: lineType, raw: true},
		{name: "position[line_range][" + anchor + "][line_code]", value: lineCode, raw: true},
	}
	if lineType == "old" {
		fields = append(fields, field{name: "position[line_range][" + anchor + "][old_line]", value: strconv.Itoa(oldLine), raw: true})
	} else {
		fields = append(fields, field{name: "position[line_range][" + anchor + "][new_line]", value: strconv.Itoa(newLine), raw: true})
	}
	return fields
}

func buildLineCode(path string, oldLine int, newLine int) string {
	sum := sha1.Sum([]byte(path))
	return hex.EncodeToString(sum[:]) + "_" + strconv.Itoa(oldLine) + "_" + strconv.Itoa(newLine)
}

func lineNumbersForSide(side domain.LineSideEnum, line int) (int, int) {
	if side == domain.LineSideOld {
		return line, 0
	}
	return 0, line
}

func lineTypeForSide(side domain.LineSideEnum) string {
	if side == domain.LineSideOld {
		return "old"
	}
	return "new"
}

func ensureLineSide(side domain.LineSideEnum) domain.LineSideEnum {
	if strings.TrimSpace(string(side)) == "" {
		return domain.LineSideNew
	}
	return side
}
