package domain

import "time"

// CommentKindEnum identifies the GitHub comment surface.
type CommentKindEnum string

const (
	// CommentKindIssue represents issue/PR conversation comments.
	CommentKindIssue CommentKindEnum = "ISSUE"
	// CommentKindReview represents pull request review comments.
	CommentKindReview CommentKindEnum = "REVIEW"
)

// CommentAuthor captures basic author metadata.
type CommentAuthor struct {
	Login string
	Type  string
}

// Comment represents a GitHub comment with minimal thread metadata.
type Comment struct {
	ID          int64
	Body        string
	Author      CommentAuthor
	CreatedAt   time.Time
	InReplyToID int64
}

// CommentThread groups the conversation context for a comment.
type CommentThread struct {
	Kind     CommentKindEnum
	RootID   int64
	Context  []string
	Comments []Comment
}
