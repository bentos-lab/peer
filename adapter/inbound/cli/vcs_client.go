package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/bentos-lab/peer/domain"
)

const (
	vcsProviderGitHub = "github"
	vcsProviderGitLab = "gitlab"
)

// VCSClient resolves repository and change request metadata.
type VCSClient interface {
	ResolveRepository(ctx context.Context, repository string) (string, error)
	GetPullRequestInfo(ctx context.Context, repository string, pullRequestNumber int) (domain.ChangeRequestInfo, error)
	GetIssue(ctx context.Context, repository string, issueNumber int) (domain.Issue, error)
	ListIssueComments(ctx context.Context, repository string, pullRequestNumber int) ([]domain.IssueComment, error)
	ListChangeRequestComments(ctx context.Context, repository string, pullRequestNumber int) ([]domain.IssueComment, error)
	GetPullRequestReview(ctx context.Context, repository string, pullRequestNumber int, reviewID int64) (domain.ReviewSummary, error)
	GetIssueComment(ctx context.Context, repository string, pullRequestNumber int, commentID int64) (domain.IssueComment, error)
	GetReviewComment(ctx context.Context, repository string, pullRequestNumber int, commentID int64) (domain.ReviewComment, error)
	ListReviewComments(ctx context.Context, repository string, pullRequestNumber int) ([]domain.ReviewComment, error)
}

// VCSClientResolver resolves a VCS client for the provided provider string.
type VCSClientResolver interface {
	Resolve(provider string) (VCSClient, error)
}

// StaticVCSClients is a simple resolver backed by fixed clients.
type StaticVCSClients struct {
	GitHub VCSClient
	GitLab VCSClient
}

// Resolve returns the configured VCS client for provider.
func (r StaticVCSClients) Resolve(provider string) (VCSClient, error) {
	provider = normalizeVCSProvider(provider)
	switch provider {
	case vcsProviderGitHub:
		if r.GitHub == nil {
			return nil, fmt.Errorf("github client is not configured")
		}
		return r.GitHub, nil
	case vcsProviderGitLab:
		if r.GitLab == nil {
			return nil, fmt.Errorf("gitlab client is not configured")
		}
		return r.GitLab, nil
	default:
		return nil, fmt.Errorf("unsupported vcs provider: %s", provider)
	}
}

func normalizeVCSProvider(provider string) string {
	provider = strings.TrimSpace(strings.ToLower(provider))
	if provider == "" {
		return vcsProviderGitHub
	}
	return provider
}
