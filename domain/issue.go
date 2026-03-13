package domain

// Issue represents a tracked issue with its core metadata.
type Issue struct {
	Repository string
	Number     int
	Title      string
	Body       string
	URL        string
}

// IssueContext provides issue details and its comments.
type IssueContext struct {
	Issue    Issue
	Comments []Comment
}
