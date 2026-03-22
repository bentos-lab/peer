package cli

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/bentos-lab/peer/domain"
	sharedtext "github.com/bentos-lab/peer/shared/text"
)

func domainChangeRequestInput(repository string, prNumber int, repoURL string, base string, head string, title string, description string, metadata map[string]string) domain.ChangeRequestInput {
	if metadata == nil {
		metadata = map[string]string{}
	}
	return domain.ChangeRequestInput{
		Target:      domain.ChangeRequestTarget{Repository: repository, ChangeRequestNumber: prNumber},
		RepoURL:     repoURL,
		Base:        base,
		Head:        head,
		Title:       title,
		Description: description,
		Language:    "English",
		Metadata:    metadata,
	}
}

func domainChangeRequestInputForAutogen(repository string, prNumber int, repoURL string, base string, head string, title string, description string) domain.ChangeRequestInput {
	return domainChangeRequestInput(repository, prNumber, repoURL, base, head, title, description, map[string]string{})
}

func resolveChangeRequestParams(ctx context.Context, vcsClient VCSClient, params ChangeRequestParams) (ChangeRequestResolution, error) {
	if vcsClient == nil {
		return ChangeRequestResolution{}, errors.New("vcs client is not configured")
	}
	provider := normalizeVCSProvider(params.VCSProvider)

	if strings.TrimSpace(params.ChangeRequest) != "" && (strings.TrimSpace(params.Base) != "" || strings.TrimSpace(params.Head) != "") {
		return ChangeRequestResolution{}, errors.New("--change-request cannot be used with --base or --head")
	}
	if params.Publish && strings.TrimSpace(params.ChangeRequest) == "" {
		return ChangeRequestResolution{}, errors.New("--publish requires --change-request")
	}

	repository, repoURL, buildRepoURL, err := normalizeRepo(provider, params.VCSHost, params.Repo)
	if err != nil {
		return ChangeRequestResolution{}, err
	}
	repoProvided := strings.TrimSpace(params.Repo) != ""
	changeRequestProvided := strings.TrimSpace(params.ChangeRequest) != ""
	if changeRequestProvided {
		repository, err = vcsClient.ResolveRepository(ctx, repository)
		if err != nil {
			return ChangeRequestResolution{}, err
		}
	} else {
		// No change request provided: skip VCS resolution.
		// Keep normalized repo URL when supplied to allow cloning into tmp.
		if !repoProvided {
			repository = "local"
			repoURL = ""
			buildRepoURL = nil
		}
	}

	base, head := resolveBaseHeadDefaults(params.Base, params.Head, repoProvided)

	prNumber := 0
	title := ""
	description := ""
	headRefName := ""
	var issueCandidates []domain.IssueContext
	if changeRequestProvided {
		parsed, parseErr := strconv.Atoi(strings.TrimSpace(params.ChangeRequest))
		if parseErr != nil || parsed <= 0 {
			return ChangeRequestResolution{}, fmt.Errorf("--change-request must be a positive integer")
		}
		prNumber = parsed
		prInfo, infoErr := vcsClient.GetPullRequestInfo(ctx, repository, prNumber)
		if infoErr != nil {
			return ChangeRequestResolution{}, infoErr
		}
		repository = prInfo.Repository
		if repoProvided && buildRepoURL != nil {
			repoURL = buildRepoURL(prInfo.Repository)
		}
		base = prInfo.BaseRef
		head = prInfo.HeadRef
		headRefName = prInfo.HeadRefName
		title = prInfo.Title
		description = prInfo.Description
		if params.IssueAlignment {
			issueCandidates = resolveIssueCandidates(ctx, provider, vcsClient, repository, description)
		}
	}
	if repoProvided && isWorkspaceHeadToken(head) {
		return ChangeRequestResolution{}, fmt.Errorf("--head %s requires local workspace mode; omit --repo", head)
	}

	return ChangeRequestResolution{
		Repository:          repository,
		RepoURL:             repoURL,
		ChangeRequestNumber: prNumber,
		Title:               title,
		Description:         description,
		Base:                base,
		Head:                head,
		HeadRefName:         headRefName,
		IssueCandidates:     issueCandidates,
	}, nil
}

func resolveIssueCandidates(ctx context.Context, provider string, vcsClient VCSClient, repository string, description string) []domain.IssueContext {
	refs := resolveIssueReferences(provider, description, repository)
	if len(refs) == 0 {
		return nil
	}

	candidates := make([]domain.IssueContext, 0, len(refs))
	for _, ref := range refs {
		issue, err := vcsClient.GetIssue(ctx, ref.Repository, ref.Number)
		if err != nil {
			continue
		}
		comments, err := vcsClient.ListIssueComments(ctx, ref.Repository, ref.Number)
		if err != nil {
			continue
		}
		issueComments := make([]domain.Comment, 0, len(comments))
		for _, comment := range comments {
			issueComments = append(issueComments, comment.ToDomain())
		}
		candidates = append(candidates, domain.IssueContext{
			Issue:    issue,
			Comments: issueComments,
		})
	}
	if len(candidates) == 0 {
		return nil
	}
	return candidates
}

func resolveIssueReferences(provider string, description string, defaultRepo string) []sharedtext.IssueReference {
	switch normalizeVCSProvider(provider) {
	case vcsProviderGitLab:
		return sharedtext.ExtractGitLabIssueReferences(description, defaultRepo)
	default:
		return sharedtext.ExtractGitHubIssueReferences(description, defaultRepo)
	}
}

func isWorkspaceHeadToken(head string) bool {
	switch strings.TrimSpace(head) {
	case "@staged", "@all":
		return true
	default:
		return false
	}
}
