package gitlab

import (
	"context"
	"errors"

	"bentos-backend/domain"
)

// Client is a placeholder GitLab API client.
type Client struct{}

// NewClient creates a GitLab API client.
func NewClient() *Client {
	return &Client{}
}

// GetMergeRequestChangedFiles loads changed files for a merge request.
func (c *Client) GetMergeRequestChangedFiles(_ context.Context, _ string, _ int) ([]domain.ChangedFile, error) {
	return nil, errors.New("gitlab diff client is not implemented yet")
}

// CreateMergeRequestNote posts an MR note to GitLab.
func (c *Client) CreateMergeRequestNote(_ context.Context, _ string, _ int, _ string) error {
	return errors.New("gitlab note client is not implemented yet")
}
