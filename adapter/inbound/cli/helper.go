package cli

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	githubvcs "bentos-backend/adapter/outbound/vcs/github"
	"bentos-backend/domain"
	sharedtext "bentos-backend/shared/text"
	"bentos-backend/shared/toolinstall"
)

func domainChangeRequestInputForAutogen(repository string, prNumber int, repoURL string, base string, head string, title string, description string) domain.ChangeRequestInput {
	return domain.ChangeRequestInput{
		Target:      domain.ChangeRequestTarget{Repository: repository, ChangeRequestNumber: prNumber},
		RepoURL:     repoURL,
		Base:        base,
		Head:        head,
		Title:       title,
		Description: description,
		Language:    "English",
		Metadata:    map[string]string{},
	}
}

func resolveChangeRequestParams(ctx context.Context, githubClient GitHubClient, params ChangeRequestParams) (ChangeRequestResolution, error) {
	provider := strings.TrimSpace(strings.ToLower(params.VCSProvider))
	if provider == "" {
		provider = "github"
	}
	if provider != "github" {
		return ChangeRequestResolution{}, fmt.Errorf("unsupported vcs provider: %s", provider)
	}

	if strings.TrimSpace(params.ChangeRequest) != "" && (strings.TrimSpace(params.Base) != "" || strings.TrimSpace(params.Head) != "") {
		return ChangeRequestResolution{}, errors.New("--change-request cannot be used with --base or --head")
	}
	if params.Publish && strings.TrimSpace(params.ChangeRequest) == "" {
		return ChangeRequestResolution{}, errors.New("--publish requires --change-request")
	}

	repository, repoURL, buildRepoURL, err := normalizeRepo(params.Repo)
	if err != nil {
		return ChangeRequestResolution{}, err
	}
	repoProvided := strings.TrimSpace(params.Repo) != ""
	repository, err = githubClient.ResolveRepository(ctx, repository)
	if err != nil {
		return ChangeRequestResolution{}, err
	}

	base := strings.TrimSpace(params.Base)
	head := strings.TrimSpace(params.Head)
	if head == "" {
		if repoProvided {
			head = "HEAD"
		} else {
			head = "@staged"
		}
	}
	if base == "" && head != "" {
		base = "HEAD"
	}

	prNumber := 0
	title := ""
	description := ""
	var issueCandidates []domain.IssueContext
	if strings.TrimSpace(params.ChangeRequest) != "" {
		parsed, parseErr := strconv.Atoi(strings.TrimSpace(params.ChangeRequest))
		if parseErr != nil || parsed <= 0 {
			return ChangeRequestResolution{}, fmt.Errorf("--change-request must be a positive integer")
		}
		prNumber = parsed
		prInfo, infoErr := githubClient.GetPullRequestInfo(ctx, repository, prNumber)
		if infoErr != nil {
			return ChangeRequestResolution{}, infoErr
		}
		repository = prInfo.Repository
		if repoProvided && buildRepoURL != nil {
			repoURL = buildRepoURL(prInfo.Repository)
		}
		base = prInfo.BaseRef
		head = prInfo.HeadRef
		title = prInfo.Title
		description = prInfo.Description
		if params.IssueAlignment {
			issueCandidates = resolveIssueCandidates(ctx, githubClient, repository, description)
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
		IssueCandidates:     issueCandidates,
	}, nil
}

func resolveIssueCandidates(ctx context.Context, githubClient GitHubClient, repository string, description string) []domain.IssueContext {
	refs := sharedtext.ExtractIssueReferences(description, repository)
	if len(refs) == 0 {
		return nil
	}

	candidates := make([]domain.IssueContext, 0, len(refs))
	for _, ref := range refs {
		issue, err := githubClient.GetIssue(ctx, ref.Repository, ref.Number)
		if err != nil {
			continue
		}
		comments, err := githubClient.ListIssueComments(ctx, ref.Repository, ref.Number)
		if err != nil {
			continue
		}
		issueComments := make([]domain.Comment, 0, len(comments))
		for _, comment := range comments {
			issueComments = append(issueComments, comment.ToDomain())
		}
		candidates = append(candidates, domain.IssueContext{
			Issue:    issue.ToDomain(),
			Comments: issueComments,
		})
	}
	if len(candidates) == 0 {
		return nil
	}
	return candidates
}

func isWorkspaceHeadToken(head string) bool {
	switch strings.TrimSpace(head) {
	case "@staged", "@all":
		return true
	default:
		return false
	}
}

func normalizeRepo(input string) (string, string, repoURLBuilder, error) {
	value := strings.TrimSpace(input)
	if value == "" {
		return "", "", nil, nil
	}

	repository, buildRepoURL, err := parseRepositorySlug(value)
	if err != nil {
		return "", "", nil, fmt.Errorf("invalid --repo value %q", input)
	}
	repoURL := buildRepoURL(repository)
	return repository, repoURL, buildRepoURL, nil
}

func parseRepositorySlug(value string) (string, repoURLBuilder, error) {
	if strings.Contains(value, "://") {
		repository, buildRepoURL, err := parseRepositoryFromURL(value)
		if err != nil {
			return "", nil, err
		}
		return repository, buildRepoURL, nil
	}

	if strings.Contains(value, "@") && strings.Contains(value, ":") {
		repository, buildRepoURL, err := parseRepositoryFromSSH(value)
		if err != nil {
			return "", nil, err
		}
		return repository, buildRepoURL, nil
	}

	repository, err := parseRepositoryPath(value)
	if err != nil {
		return "", nil, err
	}
	return repository, httpsRepoURLBuilder, nil
}

func parseRepositoryFromURL(value string) (string, repoURLBuilder, error) {
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

	repository, err := parseRepositoryPath(parsed.Path)
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

func parseRepositoryFromSSH(value string) (string, repoURLBuilder, error) {
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

	repository, err := parseRepositoryPath(parts[1])
	if err != nil {
		return "", nil, err
	}
	return repository, gitAtRepoURLBuilder, nil
}

func parseRepositoryPath(path string) (string, error) {
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

func buildInlineQuestionThread(question string, prInfo githubvcs.PullRequestInfo) domain.CommentThread {
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
	}

	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("--comment-id must be a positive integer or discussion anchor")
	}
	return parsed, nil
}

func buildIssueThread(ctx context.Context, client ReplyCommentGitHubClient, repository string, prNumber int, commentID int64, prInfo githubvcs.PullRequestInfo) (domain.CommentThread, error) {
	comments, err := client.ListIssueComments(ctx, repository, prNumber)
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

func buildReviewThread(ctx context.Context, client ReplyCommentGitHubClient, repository string, prNumber int, commentID int64) (domain.CommentThread, error) {
	comments, err := client.ListReviewComments(ctx, repository, prNumber)
	if err != nil {
		return domain.CommentThread{}, err
	}

	byID := make(map[int64]githubvcs.ReviewComment, len(comments))
	for _, comment := range comments {
		byID[comment.ID] = comment
	}

	rootID := resolveReviewRootID(byID, commentID)
	threadComments := make([]domain.Comment, 0, len(comments))
	var root githubvcs.ReviewComment
	if comment, ok := byID[rootID]; ok {
		root = comment
	}
	reviewSummary := githubvcs.PullRequestReviewSummary{}
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

func resolveReviewRootID(byID map[int64]githubvcs.ReviewComment, commentID int64) int64 {
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

func buildIssueThreadContext(prInfo githubvcs.PullRequestInfo) []string {
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

func buildReviewThreadContext(root githubvcs.ReviewComment, reviewSummary githubvcs.PullRequestReviewSummary) []string {
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

func formatReviewLineInfo(root githubvcs.ReviewComment) string {
	if root.Line > 0 {
		return fmt.Sprintf("Line: %d (%s)", root.Line, strings.TrimSpace(root.Side))
	}
	if root.OriginalLine > 0 {
		return fmt.Sprintf("Original Line: %d", root.OriginalLine)
	}
	return ""
}

func formatReviewSummary(summary githubvcs.PullRequestReviewSummary) string {
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

func (c *InstallCommand) resolveInstaller() ToolInstaller {
	if c == nil {
		return &missingInstaller{}
	}
	if c.installer == nil {
		c.installer = toolinstall.NewInstaller(toolinstall.Config{})
	}
	return c.installer
}
