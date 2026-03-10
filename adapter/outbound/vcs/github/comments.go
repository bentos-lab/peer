package github

import (
	"strings"
	"time"

	"bentos-backend/domain"
)

// IssueComment contains normalized PR issue comment data.
type IssueComment struct {
	ID        int64
	Body      string
	Author    CommentAuthor
	CreatedAt time.Time
}

// ReviewComment contains normalized PR review comment data.
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

// CommentAuthor captures author identity for comments.
type CommentAuthor struct {
	Login string
	Type  string
}

func (c IssueComment) ToDomain() domain.Comment {
	return domain.Comment{
		ID:        c.ID,
		Body:      strings.TrimSpace(c.Body),
		Author:    domain.CommentAuthor{Login: strings.TrimSpace(c.Author.Login), Type: strings.TrimSpace(c.Author.Type)},
		CreatedAt: c.CreatedAt,
	}
}

func (c ReviewComment) ToDomain() domain.Comment {
	return domain.Comment{
		ID:          c.ID,
		Body:        strings.TrimSpace(c.Body),
		Author:      domain.CommentAuthor{Login: strings.TrimSpace(c.Author.Login), Type: strings.TrimSpace(c.Author.Type)},
		CreatedAt:   c.CreatedAt,
		InReplyToID: c.InReplyToID,
	}
}
