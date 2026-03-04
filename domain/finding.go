package domain

// FindingSeverityEnum defines review finding severity.
type FindingSeverityEnum string

const (
	// FindingSeverityCritical indicates high impact potential issues.
	FindingSeverityCritical FindingSeverityEnum = "CRITICAL"
	// FindingSeverityMajor indicates medium impact potential issues.
	FindingSeverityMajor FindingSeverityEnum = "MAJOR"
	// FindingSeverityMinor indicates low impact potential issues.
	FindingSeverityMinor FindingSeverityEnum = "MINOR"
	// FindingSeverityNit indicates style or tiny improvements.
	FindingSeverityNit FindingSeverityEnum = "NIT"
)

// Finding represents one LLM-generated review finding.
type Finding struct {
	FilePath   string
	StartLine  int
	EndLine    int
	Severity   FindingSeverityEnum
	Title      string
	Detail     string
	Suggestion string
}
