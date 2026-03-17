package domain

// LineSideEnum identifies whether a line refers to the new or old file.
type LineSideEnum string

const (
	// LineSideNew refers to lines on the new (right) side of a diff.
	LineSideNew LineSideEnum = "NEW"
	// LineSideOld refers to lines on the old (left) side of a diff.
	LineSideOld LineSideEnum = "OLD"
)
