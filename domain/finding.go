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

// SuggestedChangeKindEnum defines the suggestion operation kind.
type SuggestedChangeKindEnum string

const (
	// SuggestedChangeKindReplace indicates replacing selected lines with new content.
	SuggestedChangeKindReplace SuggestedChangeKindEnum = "REPLACE"
	// SuggestedChangeKindDelete indicates removing selected lines.
	SuggestedChangeKindDelete SuggestedChangeKindEnum = "DELETE"
)

// SuggestedChange represents a machine-generated code change proposal.
type SuggestedChange struct {
	Replacement string
	Kind        SuggestedChangeKindEnum
	Reason      string
}

// Finding represents one LLM-generated review finding.
type Finding struct {
	FilePath        string
	StartLine       int
	EndLine         int
	Severity        FindingSeverityEnum
	Title           string
	Detail          string
	Suggestion      string
	SuggestedChange *SuggestedChange
}
