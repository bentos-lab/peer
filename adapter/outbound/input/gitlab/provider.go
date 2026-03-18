package gitlab

import (
	"context"

	"bentos-backend/domain"
	"bentos-backend/usecase"
)

// DiffClient loads MR changed files from GitLab.
type DiffClient interface {
	GetMergeRequestChangedFiles(ctx context.Context, repository string, mergeRequestNumber int) ([]domain.ChangedFile, error)
}

// Provider loads review input from GitLab merge request data.
type Provider struct {
	client DiffClient
}

// NewProvider creates a GitLab review input provider.
func NewProvider(client DiffClient) *Provider {
	return &Provider{client: client}
}

// LoadReviewInput loads changed contents from a GitLab merge request.
func (p *Provider) LoadReviewInput(ctx context.Context, request usecase.ReviewRequest) (domain.ReviewInput, error) {
	files, err := p.client.GetMergeRequestChangedFiles(ctx, request.Repository, request.ChangeRequestNumber)
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
