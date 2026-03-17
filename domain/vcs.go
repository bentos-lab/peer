package domain

import (
	"errors"
	"time"
)

// ReviewCommentInput contains one anchored review comment payload.
type ReviewCommentInput struct {
	Body      string
	Path      string
	StartLine int
	EndLine   int
	LineSide  LineSideEnum
}

// ChangeRequestInfo contains normalized change request metadata.
type ChangeRequestInfo struct {
	Repository  string
	Number      int
	Title       string
	Description string
	BaseRef     string
	HeadRef     string
	HeadRefName string
	StartRef    string
}

// ReviewSummary contains normalized review summary metadata.
type ReviewSummary struct {
	ID    int64
	Body  string
	State string
	User  CommentAuthor
}

// IssueComment contains normalized issue comment data.
type IssueComment struct {
	ID        int64
	Body      string
	Author    CommentAuthor
	CreatedAt time.Time
}

// ToDomain maps the issue comment into a generic domain comment.
func (c IssueComment) ToDomain() Comment {
	return Comment{
		ID:        c.ID,
		Body:      c.Body,
		Author:    CommentAuthor{Login: c.Author.Login, Type: c.Author.Type},
		CreatedAt: c.CreatedAt,
	}
}

// ReviewComment contains normalized review comment data.
type ReviewComment struct {
	ID                int64
	Body              string
	Author            CommentAuthor
	CreatedAt         time.Time
	InReplyToID       int64
	Path              string
	DiffHunk          string
	Line              int
	OriginalLine      int
	StartLine         int
	OriginalStartLine int
	Side              string
	StartSide         string
	CommitID          string
	ReviewID          int64
}

// ToDomain maps the review comment into a generic domain comment.
func (c ReviewComment) ToDomain() Comment {
	return Comment{
		ID:          c.ID,
		Body:        c.Body,
		Author:      CommentAuthor{Login: c.Author.Login, Type: c.Author.Type},
		CreatedAt:   c.CreatedAt,
		InReplyToID: c.InReplyToID,
	}
}

// InvalidAnchorError means the VCS rejected the requested file/line anchor.
type InvalidAnchorError struct {
	Message string
	Cause   error
}

// Error returns the invalid-anchor error message.
func (e *InvalidAnchorError) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause == nil {
		return e.Message
	}
	return e.Message + ": " + e.Cause.Error()
}

// Unwrap returns the underlying cause.
func (e *InvalidAnchorError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// IsInvalidAnchorError reports whether err wraps InvalidAnchorError.
func IsInvalidAnchorError(err error) bool {
	var invalidAnchorErr *InvalidAnchorError
	return errors.As(err, &invalidAnchorErr)
}
