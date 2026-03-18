package domain

// ReviewMessageTypeEnum describes the message purpose.
type ReviewMessageTypeEnum string

const (
	// ReviewMessageTypeFileGroup is a grouped message for one file/area.
	ReviewMessageTypeFileGroup ReviewMessageTypeEnum = "FILE_GROUP"
	// ReviewMessageTypeSummary is the short summary message.
	ReviewMessageTypeSummary ReviewMessageTypeEnum = "SUMMARY"
)

// ReviewMessage is a publishable review message.
type ReviewMessage struct {
	Type         ReviewMessageTypeEnum
	Title        string
	Body         string
	FilePath     string
	FindingCount int
}
