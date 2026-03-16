package text

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// IssueReference identifies an issue reference found in free-form text.
type IssueReference struct {
	Repository string
	Number     int
}

type issueMatch struct {
	start int
	end   int
	ref   IssueReference
}

var (
	issueURLPattern      = regexp.MustCompile(`https?://github\.com/([A-Za-z0-9_.-]+)/([A-Za-z0-9_.-]+)/issues/([0-9]+)`)
	issueRepoRefPattern  = regexp.MustCompile(`([A-Za-z0-9_.-]+)/([A-Za-z0-9_.-]+)#([0-9]+)`)
	issueShortRefPattern = regexp.MustCompile(`(^|[^A-Za-z0-9_/.])#([0-9]+)`)

	gitlabIssueURLPattern     = regexp.MustCompile(`https?://[^/\s]+/([A-Za-z0-9_.-]+(?:/[A-Za-z0-9_.-]+)+)/-/issues/([0-9]+)`)
	gitlabIssueRepoRefPattern = regexp.MustCompile(`([A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+(?:/[A-Za-z0-9_.-]+)*)#([0-9]+)`)
)

// ExtractIssueReferences finds GitHub issue references in the provided description.
// defaultRepo is used for short-form references like #123.
func ExtractIssueReferences(description string, defaultRepo string) []IssueReference {
	return ExtractGitHubIssueReferences(description, defaultRepo)
}

// ExtractGitHubIssueReferences finds GitHub issue references in the provided description.
// defaultRepo is used for short-form references like #123.
func ExtractGitHubIssueReferences(description string, defaultRepo string) []IssueReference {
	description = strings.TrimSpace(description)
	if description == "" {
		return nil
	}

	matches := make([]issueMatch, 0)
	addMatch := func(start int, end int, ref IssueReference) {
		if ref.Number <= 0 {
			return
		}
		if strings.TrimSpace(ref.Repository) == "" {
			return
		}
		matches = append(matches, issueMatch{start: start, end: end, ref: ref})
	}

	for _, idx := range issueURLPattern.FindAllStringSubmatchIndex(description, -1) {
		if len(idx) < 8 {
			continue
		}
		owner := description[idx[2]:idx[3]]
		repo := description[idx[4]:idx[5]]
		number, _ := strconv.Atoi(description[idx[6]:idx[7]])
		addMatch(idx[0], idx[1], IssueReference{Repository: owner + "/" + repo, Number: number})
	}

	for _, idx := range issueRepoRefPattern.FindAllStringSubmatchIndex(description, -1) {
		if len(idx) < 8 {
			continue
		}
		owner := description[idx[2]:idx[3]]
		repo := description[idx[4]:idx[5]]
		number, _ := strconv.Atoi(description[idx[6]:idx[7]])
		addMatch(idx[0], idx[1], IssueReference{Repository: owner + "/" + repo, Number: number})
	}

	if strings.TrimSpace(defaultRepo) != "" {
		for _, idx := range issueShortRefPattern.FindAllStringSubmatchIndex(description, -1) {
			if len(idx) < 6 {
				continue
			}
			number, _ := strconv.Atoi(description[idx[4]:idx[5]])
			addMatch(idx[2], idx[5], IssueReference{Repository: defaultRepo, Number: number})
		}
	}

	return normalizeIssueMatches(matches)
}

// ExtractGitLabIssueReferences finds GitLab issue references in the provided description.
// defaultRepo is used for short-form references like #123.
func ExtractGitLabIssueReferences(description string, defaultRepo string) []IssueReference {
	description = strings.TrimSpace(description)
	if description == "" {
		return nil
	}

	matches := make([]issueMatch, 0)
	addMatch := func(start int, end int, ref IssueReference) {
		if ref.Number <= 0 {
			return
		}
		if strings.TrimSpace(ref.Repository) == "" {
			return
		}
		matches = append(matches, issueMatch{start: start, end: end, ref: ref})
	}

	for _, idx := range gitlabIssueURLPattern.FindAllStringSubmatchIndex(description, -1) {
		if len(idx) < 6 {
			continue
		}
		repo := description[idx[2]:idx[3]]
		number, _ := strconv.Atoi(description[idx[4]:idx[5]])
		addMatch(idx[0], idx[1], IssueReference{Repository: repo, Number: number})
	}

	for _, idx := range gitlabIssueRepoRefPattern.FindAllStringSubmatchIndex(description, -1) {
		if len(idx) < 6 {
			continue
		}
		repo := description[idx[2]:idx[3]]
		number, _ := strconv.Atoi(description[idx[4]:idx[5]])
		addMatch(idx[0], idx[1], IssueReference{Repository: repo, Number: number})
	}

	if strings.TrimSpace(defaultRepo) != "" {
		for _, idx := range issueShortRefPattern.FindAllStringSubmatchIndex(description, -1) {
			if len(idx) < 6 {
				continue
			}
			number, _ := strconv.Atoi(description[idx[4]:idx[5]])
			addMatch(idx[2], idx[5], IssueReference{Repository: defaultRepo, Number: number})
		}
	}

	return normalizeIssueMatches(matches)
}

func normalizeIssueMatches(matches []issueMatch) []IssueReference {
	if len(matches) == 0 {
		return nil
	}

	sort.Slice(matches, func(i, j int) bool {
		if matches[i].start == matches[j].start {
			return matches[i].end < matches[j].end
		}
		return matches[i].start < matches[j].start
	})

	seen := make(map[string]struct{}, len(matches))
	result := make([]IssueReference, 0, len(matches))
	for _, match := range matches {
		key := match.ref.Repository + "#" + strconv.Itoa(match.ref.Number)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, match.ref)
	}
	return result
}
