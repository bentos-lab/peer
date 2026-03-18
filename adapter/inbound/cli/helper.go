package cli

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"bentos-backend/domain"
	sharedtext "bentos-backend/shared/text"
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
	repository, err = vcsClient.ResolveRepository(ctx, repository)
	if err != nil {
		return ChangeRequestResolution{}, err
	}

	base, head := resolveBaseHeadDefaults(params.Base, params.Head, repoProvided)

	prNumber := 0
	title := ""
	description := ""
	headRefName := ""
	var issueCandidates []domain.IssueContext
	if strings.TrimSpace(params.ChangeRequest) != "" {
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

type repoURLBuilder func(repository string) string

func normalizeRepo(provider string, gitlabHostOverride string, input string) (string, string, repoURLBuilder, error) {
	value := strings.TrimSpace(input)
	if value == "" {
		return "", "", nil, nil
	}

	provider = normalizeVCSProvider(provider)
	switch provider {
	case vcsProviderGitLab:
		return normalizeGitLabRepo(value, gitlabHostOverride)
	default:
		return normalizeGitHubRepo(value)
	}
}

func normalizeGitHubRepo(value string) (string, string, repoURLBuilder, error) {
	repository, buildRepoURL, err := parseRepositorySlugGitHub(value)
	if err != nil {
		return "", "", nil, fmt.Errorf("invalid --repo value %q", value)
	}
	repoURL := buildRepoURL(repository)
	return repository, repoURL, buildRepoURL, nil
}

func normalizeGitLabRepo(value string, gitlabHostOverride string) (string, string, repoURLBuilder, error) {
	repository, buildRepoURL, err := parseRepositorySlugGitLab(value, gitlabHostOverride)
	if err != nil {
		return "", "", nil, fmt.Errorf("invalid --repo value %q", value)
	}
	repoURL := ""
	if buildRepoURL != nil {
		repoURL = buildRepoURL(repository)
	}
	return repository, repoURL, buildRepoURL, nil
}

func parseRepositorySlugGitHub(value string) (string, repoURLBuilder, error) {
	if strings.Contains(value, "://") {
		repository, buildRepoURL, err := parseRepositoryFromURLGitHub(value)
		if err != nil {
			return "", nil, err
		}
		return repository, buildRepoURL, nil
	}

	if strings.Contains(value, "@") && strings.Contains(value, ":") {
		repository, buildRepoURL, err := parseRepositoryFromSSHGithub(value)
		if err != nil {
			return "", nil, err
		}
		return repository, buildRepoURL, nil
	}

	repository, err := parseRepositoryPathGitHub(value)
	if err != nil {
		return "", nil, err
	}
	return repository, httpsRepoURLBuilder, nil
}

func parseRepositorySlugGitLab(value string, gitlabHostOverride string) (string, repoURLBuilder, error) {
	if strings.Contains(value, "://") {
		repository, buildRepoURL, err := parseRepositoryFromURLGitLab(value)
		if err != nil {
			return "", nil, err
		}
		return repository, buildRepoURL, nil
	}

	if strings.Contains(value, "@") && strings.Contains(value, ":") {
		repository, buildRepoURL, err := parseRepositoryFromSSHGitLab(value)
		if err != nil {
			return "", nil, err
		}
		return repository, buildRepoURL, nil
	}

	repository, err := parseRepositoryPathGitLab(value)
	if err != nil {
		return "", nil, err
	}
	host := gitlabHostForRepo(gitlabHostOverride)
	return repository, gitlabHTTPSRepoURLBuilder(host), nil
}

func parseRepositoryFromURLGitHub(value string) (string, repoURLBuilder, error) {
	parsed, err := url.Parse(value)
	if err != nil {
		return "", nil, err
	}
	if parsed.Host != "github.com" {
		return "", nil, fmt.Errorf("unsupported repository host")
	}
	switch parsed.Scheme {
	case "http", "https", "ssh":
	default:
		return "", nil, fmt.Errorf("unsupported repository scheme")
	}

	repository, err := parseRepositoryPathGitHub(parsed.Path)
	if err != nil {
		return "", nil, err
	}
	switch parsed.Scheme {
	case "http":
		return repository, httpRepoURLBuilder, nil
	case "https":
		return repository, httpsRepoURLBuilder, nil
	case "ssh":
		if parsed.User != nil && parsed.User.Username() == "git" {
			return repository, sshRepoURLBuilder, nil
		}
		return "", nil, fmt.Errorf("unsupported ssh repository user")
	default:
		return "", nil, fmt.Errorf("unsupported repository scheme")
	}
}

func parseRepositoryFromURLGitLab(value string) (string, repoURLBuilder, error) {
	parsed, err := url.Parse(value)
	if err != nil {
		return "", nil, err
	}
	switch parsed.Scheme {
	case "http", "https", "ssh":
	default:
		return "", nil, fmt.Errorf("unsupported repository scheme")
	}

	path := strings.TrimSpace(parsed.Path)
	if strings.Contains(path, "/-/") {
		parts := strings.SplitN(path, "/-/", 2)
		if len(parts) > 0 {
			path = parts[0]
		}
	}
	repository, err := parseRepositoryPathGitLab(path)
	if err != nil {
		return "", nil, err
	}
	host := strings.TrimSpace(parsed.Host)
	if host == "" {
		return "", nil, fmt.Errorf("unsupported repository host")
	}
	switch parsed.Scheme {
	case "http":
		return repository, gitlabHTTPRepoURLBuilder(host), nil
	case "https":
		return repository, gitlabHTTPSRepoURLBuilder(host), nil
	case "ssh":
		if parsed.User != nil && parsed.User.Username() == "git" {
			return repository, gitlabSSHRepoURLBuilder(host), nil
		}
		return "", nil, fmt.Errorf("unsupported ssh repository user")
	default:
		return "", nil, fmt.Errorf("unsupported repository scheme")
	}
}

func parseRepositoryFromSSHGithub(value string) (string, repoURLBuilder, error) {
	parts := strings.SplitN(value, ":", 2)
	if len(parts) != 2 {
		return "", nil, fmt.Errorf("invalid ssh repository")
	}

	hostParts := strings.SplitN(parts[0], "@", 2)
	if len(hostParts) != 2 || hostParts[1] != "github.com" {
		return "", nil, fmt.Errorf("unsupported repository host")
	}
	if hostParts[0] != "git" {
		return "", nil, fmt.Errorf("unsupported ssh repository user")
	}

	repository, err := parseRepositoryPathGitHub(parts[1])
	if err != nil {
		return "", nil, err
	}
	return repository, gitAtRepoURLBuilder, nil
}

func parseRepositoryFromSSHGitLab(value string) (string, repoURLBuilder, error) {
	parts := strings.SplitN(value, ":", 2)
	if len(parts) != 2 {
		return "", nil, fmt.Errorf("invalid ssh repository")
	}

	hostParts := strings.SplitN(parts[0], "@", 2)
	if len(hostParts) != 2 {
		return "", nil, fmt.Errorf("unsupported repository host")
	}
	if hostParts[0] != "git" {
		return "", nil, fmt.Errorf("unsupported ssh repository user")
	}
	host := strings.TrimSpace(hostParts[1])
	if host == "" {
		return "", nil, fmt.Errorf("unsupported repository host")
	}

	repository, err := parseRepositoryPathGitLab(parts[1])
	if err != nil {
		return "", nil, err
	}
	return repository, gitlabGitAtRepoURLBuilder(host), nil
}

func parseRepositoryPathGitHub(path string) (string, error) {
	trimmed := strings.Trim(strings.TrimSuffix(path, ".git"), "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) != 2 {
		return "", fmt.Errorf("repository must use owner/repo format")
	}
	if strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return "", fmt.Errorf("repository owner and name are required")
	}
	return parts[0] + "/" + parts[1], nil
}

func parseRepositoryPathGitLab(path string) (string, error) {
	trimmed := strings.Trim(strings.TrimSuffix(path, ".git"), "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) < 2 {
		return "", fmt.Errorf("repository path must use group/project format")
	}
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			return "", fmt.Errorf("repository group and name are required")
		}
	}
	return strings.Join(parts, "/"), nil
}

func httpRepoURLBuilder(repository string) string {
	return fmt.Sprintf("http://github.com/%s.git", strings.TrimSpace(repository))
}

func httpsRepoURLBuilder(repository string) string {
	return fmt.Sprintf("https://github.com/%s.git", strings.TrimSpace(repository))
}

func sshRepoURLBuilder(repository string) string {
	return fmt.Sprintf("ssh://git@github.com/%s.git", strings.TrimSpace(repository))
}

func gitAtRepoURLBuilder(repository string) string {
	return fmt.Sprintf("git@github.com:%s.git", strings.TrimSpace(repository))
}

func gitlabHTTPRepoURLBuilder(host string) repoURLBuilder {
	host = strings.TrimSpace(host)
	return func(repository string) string {
		return fmt.Sprintf("http://%s/%s.git", host, strings.TrimSpace(repository))
	}
}

func gitlabHTTPSRepoURLBuilder(host string) repoURLBuilder {
	host = strings.TrimSpace(host)
	return func(repository string) string {
		return fmt.Sprintf("https://%s/%s.git", host, strings.TrimSpace(repository))
	}
}

func gitlabSSHRepoURLBuilder(host string) repoURLBuilder {
	host = strings.TrimSpace(host)
	return func(repository string) string {
		return fmt.Sprintf("ssh://git@%s/%s.git", host, strings.TrimSpace(repository))
	}
}

func gitlabGitAtRepoURLBuilder(host string) repoURLBuilder {
	host = strings.TrimSpace(host)
	return func(repository string) string {
		return fmt.Sprintf("git@%s:%s.git", host, strings.TrimSpace(repository))
	}
}

func gitlabHostForRepo(override string) string {
	host := strings.TrimSpace(override)
	if host == "" {
		host = strings.TrimSpace(os.Getenv("GITLAB_HOST"))
	}
	if host == "" {
		host = "gitlab.com"
	}
	return host
}

func buildInlineQuestionThread(question string, prInfo domain.ChangeRequestInfo) domain.CommentThread {
	now := time.Now()
	return domain.CommentThread{
		Kind:    domain.CommentKindIssue,
		RootID:  0,
		Context: buildIssueThreadContext(prInfo),
		Comments: []domain.Comment{{
			ID:        0,
			Body:      strings.TrimSpace(question),
			Author:    domain.CommentAuthor{Login: "cli"},
			CreatedAt: now,
		}},
	}
}

func parseCommentID(raw string) (int64, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, fmt.Errorf("--comment-id must be a positive integer")
	}

	if strings.Contains(value, "#") {
		parts := strings.SplitN(value, "#", 2)
		if len(parts) == 2 {
			value = parts[1]
		}
	}

	switch {
	case strings.HasPrefix(value, "discussion_r"):
		value = strings.TrimPrefix(value, "discussion_r")
	case strings.HasPrefix(value, "issuecomment-"):
		value = strings.TrimPrefix(value, "issuecomment-")
	case strings.HasPrefix(value, "note_"):
		value = strings.TrimPrefix(value, "note_")
	case strings.HasPrefix(value, "note-"):
		value = strings.TrimPrefix(value, "note-")
	}

	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("--comment-id must be a positive integer or discussion anchor")
	}
	return parsed, nil
}

func buildIssueThread(ctx context.Context, client VCSClient, repository string, prNumber int, commentID int64, prInfo domain.ChangeRequestInfo) (domain.CommentThread, error) {
	comments, err := client.ListChangeRequestComments(ctx, repository, prNumber)
	if err != nil {
		return domain.CommentThread{}, err
	}
	threadComments := make([]domain.Comment, 0, len(comments))
	for _, comment := range comments {
		threadComments = append(threadComments, comment.ToDomain())
	}
	sort.Slice(threadComments, func(i, j int) bool {
		return threadComments[i].CreatedAt.Before(threadComments[j].CreatedAt)
	})
	return domain.CommentThread{
		Kind:     domain.CommentKindIssue,
		RootID:   commentID,
		Context:  buildIssueThreadContext(prInfo),
		Comments: threadComments,
	}, nil
}

func buildReviewThread(ctx context.Context, client VCSClient, repository string, prNumber int, commentID int64) (domain.CommentThread, error) {
	comments, err := client.ListReviewComments(ctx, repository, prNumber)
	if err != nil {
		return domain.CommentThread{}, err
	}

	byID := make(map[int64]domain.ReviewComment, len(comments))
	for _, comment := range comments {
		byID[comment.ID] = comment
	}

	rootID := resolveReviewRootID(byID, commentID)
	threadComments := make([]domain.Comment, 0, len(comments))
	var root domain.ReviewComment
	if comment, ok := byID[rootID]; ok {
		root = comment
	}
	reviewSummary := domain.ReviewSummary{}
	if root.ReviewID > 0 {
		if summary, err := client.GetPullRequestReview(ctx, repository, prNumber, root.ReviewID); err == nil {
			reviewSummary = summary
		}
	}
	for _, comment := range comments {
		if resolveReviewRootID(byID, comment.ID) == rootID {
			threadComments = append(threadComments, comment.ToDomain())
		}
	}
	sort.Slice(threadComments, func(i, j int) bool {
		return threadComments[i].CreatedAt.Before(threadComments[j].CreatedAt)
	})
	return domain.CommentThread{
		Kind:     domain.CommentKindReview,
		RootID:   rootID,
		Context:  buildReviewThreadContext(root, reviewSummary),
		Comments: threadComments,
	}, nil
}

func resolveReviewRootID(byID map[int64]domain.ReviewComment, commentID int64) int64 {
	currentID := commentID
	for {
		comment, ok := byID[currentID]
		if !ok || comment.InReplyToID == 0 {
			return currentID
		}
		currentID = comment.InReplyToID
	}
}

// ResolveBool returns the first non-nil bool pointer value or the default value if none are set.
func ResolveBool(primary *bool, fallback *bool, defaultValue bool) bool {
	if primary != nil {
		return *primary
	}
	if fallback != nil {
		return *fallback
	}
	return defaultValue
}

func buildIssueThreadContext(prInfo domain.ChangeRequestInfo) []string {
	title := strings.TrimSpace(prInfo.Title)
	description := strings.TrimSpace(prInfo.Description)
	if title == "" && description == "" {
		return nil
	}
	lines := []string{"PR Description:"}
	if title != "" {
		lines = append(lines, fmt.Sprintf("Title: %s", title))
	}
	if description != "" {
		lines = append(lines, description)
	}
	return lines
}

func buildReviewThreadContext(root domain.ReviewComment, reviewSummary domain.ReviewSummary) []string {
	lines := make([]string, 0)
	if strings.TrimSpace(root.Path) != "" {
		lines = append(lines, fmt.Sprintf("File: %s", strings.TrimSpace(root.Path)))
	}
	lineInfo := formatReviewLineInfo(root)
	if lineInfo != "" {
		lines = append(lines, lineInfo)
	}
	if strings.TrimSpace(root.DiffHunk) != "" {
		lines = append(lines, "Diff Hunk:", "```diff", root.DiffHunk, "```")
	}
	if summary := formatReviewSummary(reviewSummary); summary != "" {
		lines = append(lines, "Review Summary:", summary)
	}
	if len(lines) == 0 {
		return nil
	}
	return lines
}

func formatReviewLineInfo(root domain.ReviewComment) string {
	if root.Line > 0 {
		return fmt.Sprintf("Line: %d (%s)", root.Line, strings.TrimSpace(root.Side))
	}
	if root.OriginalLine > 0 {
		return fmt.Sprintf("Original Line: %d", root.OriginalLine)
	}
	return ""
}

func formatReviewSummary(summary domain.ReviewSummary) string {
	body := strings.TrimSpace(summary.Body)
	if body == "" {
		return ""
	}
	state := strings.TrimSpace(summary.State)
	author := strings.TrimSpace(summary.User.Login)
	if state != "" || author != "" {
		prefix := "Review"
		if state != "" {
			prefix = fmt.Sprintf("%s (%s)", prefix, state)
		}
		if author != "" {
			prefix = fmt.Sprintf("%s by %s", prefix, author)
		}
		return fmt.Sprintf("%s:\n%s", prefix, body)
	}
	return body
}
