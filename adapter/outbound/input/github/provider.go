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

// LoadReviewInput loads changed contents from a GitHub pull request.
func (p *Provider) LoadReviewInput(ctx context.Context, request usecase.ReviewRequest) (domain.ReviewInput, error) {
	files, err := p.client.GetPullRequestChangedFiles(ctx, request.Repository, request.ChangeRequestNumber)
	if err != nil {
		return domain.ReviewInput{}, err
	}
	return domain.ReviewInput{
		ReviewID: request.ReviewID,
		Target: domain.ReviewTarget{
			Repository:          request.Repository,
			ChangeRequestNumber: request.ChangeRequestNumber,
		},
		Title:        request.Title,
		Description:  request.Description,
		ChangedFiles: files,
		Language:     "English",
		Metadata:     request.Metadata,
	}, nil
}
