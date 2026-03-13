package github

import (
	"strings"

	"bentos-backend/domain"
)

// Issue contains normalized GitHub issue metadata.
type Issue struct {
	Repository string
	Number     int
	Title      string
	Body       string
	URL        string
}

// ToDomain maps the GitHub issue payload into a domain issue.
func (i Issue) ToDomain() domain.Issue {
	return domain.Issue{
		Repository: strings.TrimSpace(i.Repository),
		Number:     i.Number,
		Title:      strings.TrimSpace(i.Title),
		Body:       strings.TrimSpace(i.Body),
		URL:        strings.TrimSpace(i.URL),
	}
}
