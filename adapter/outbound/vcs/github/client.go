package github

import (
	"context"
	"errors"

	"bentos-backend/domain"
)

// Client is a placeholder GitHub API client.
type Client struct{}

// NewClient creates a GitHub API client.
func NewClient() *Client {
	return &Client{}
}

// GetPullRequestChangedFiles loads changed files for a pull request.
func (c *Client) GetPullRequestChangedFiles(_ context.Context, _ string, _ int) ([]domain.ChangedFile, error) {
	return nil, errors.New("github diff client is not implemented yet")
}

// CreateComment posts a comment to GitHub.
func (c *Client) CreateComment(_ context.Context, _ string, _ int, _ string) error {
	return errors.New("github comment client is not implemented yet")
}
