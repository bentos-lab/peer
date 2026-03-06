package github

import (
	"context"

	"bentos-backend/domain"
	"bentos-backend/usecase"
)

// DiffClient loads PR changed files from GitHub.
type DiffClient interface {
	GetPullRequestChangedFiles(ctx context.Context, repository string, pullRequestNumber int) ([]domain.ChangedFile, error)
}

// Provider loads review input from GitHub pull request data.
type Provider struct {
	client DiffClient
}

// NewProvider creates a GitHub review input provider.
func NewProvider(client DiffClient) *Provider {
	return &Provider{client: client}
}

// LoadChangeSnapshot loads changed contents from a GitHub pull request.
func (p *Provider) LoadChangeSnapshot(ctx context.Context, request usecase.ChangeRequestRequest) (domain.ChangeSnapshot, error) {
	files, err := p.client.GetPullRequestChangedFiles(ctx, request.Repository, request.ChangeRequestNumber)
	if err != nil {
		return domain.ChangeSnapshot{}, err
	}
	return domain.ChangeSnapshot{
		Context: domain.ChangeRequestContext{
			Repository:          request.Repository,
			ChangeRequestNumber: request.ChangeRequestNumber,
			Title:               request.Title,
			Description:         request.Description,
			Metadata:            request.Metadata,
		},
		ChangedFiles: files,
		Language:     "English",
	}, nil
}
