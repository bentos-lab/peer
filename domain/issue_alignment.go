package domain

// IssueAlignmentResult captures how well the change request aligns to a linked issue.
type IssueAlignmentResult struct {
	Issue        IssueReference
	KeyIdeas     []string
	Requirements []IssueAlignmentRequirement
}

// IssueAlignmentRequirement describes one requirement and its coverage status.
type IssueAlignmentRequirement struct {
	Requirement string
	Coverage    string
}

// IssueReference identifies the linked issue used for alignment.
type IssueReference struct {
	Repository string
	Number     int
	Title      string
}
