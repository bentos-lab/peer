package logging

import (
	"net/url"
	"sort"
	"strings"

	"bentos-backend/shared/logger/stdlogger"
	"bentos-backend/usecase"
)

// LogInputSnapshot logs a safe summary of inbound parameters before usecase execution.
func LogInputSnapshot(logger usecase.Logger, source string, action string, request any) {
	logger = ensureLogger(logger)

	switch typed := request.(type) {
	case usecase.ReviewRequest:
		logReviewSnapshot(logger, source, action, typed)
	case *usecase.ReviewRequest:
		if typed != nil {
			logReviewSnapshot(logger, source, action, *typed)
		}
	case usecase.OverviewRequest:
		logOverviewSnapshot(logger, source, action, typed)
	case *usecase.OverviewRequest:
		if typed != nil {
			logOverviewSnapshot(logger, source, action, *typed)
		}
	case usecase.AutogenRequest:
		logAutogenSnapshot(logger, source, action, typed)
	case *usecase.AutogenRequest:
		if typed != nil {
			logAutogenSnapshot(logger, source, action, *typed)
		}
	case usecase.ReplyCommentRequest:
		logReplyCommentSnapshot(logger, source, action, typed)
	case *usecase.ReplyCommentRequest:
		if typed != nil {
			logReplyCommentSnapshot(logger, source, action, *typed)
		}
	}
}

func logReviewSnapshot(logger usecase.Logger, source string, action string, request usecase.ReviewRequest) {
	trimmedAction := strings.TrimSpace(action)
	repo := request.Input.Target.Repository
	number := request.Input.Target.ChangeRequestNumber
	if trimmedAction == "" {
		logger.Infof(
			"Pre-usecase input snapshot source=%q repository=%q changeRequestNumber=%d suggestions=%t.",
			strings.TrimSpace(source),
			repo,
			number,
			request.Suggestions,
		)
	} else {
		logger.Infof(
			"Pre-usecase input snapshot source=%q action=%q repository=%q changeRequestNumber=%d suggestions=%t.",
			strings.TrimSpace(source),
			trimmedAction,
			repo,
			number,
			request.Suggestions,
		)
	}

	safeRepoURL, hasRepoURL := sanitizeRepoURL(request.Input.RepoURL)
	logger.Debugf(
		"Pre-usecase input details source=%q action=%q base=%q head=%q metadataKeys=%q metadataCount=%d titleLength=%d descriptionLength=%d repoURLPresent=%t repoURLSafe=%q.",
		strings.TrimSpace(source),
		trimmedAction,
		request.Input.Base,
		request.Input.Head,
		strings.Join(sortedMetadataKeys(request.Input.Metadata), ","),
		len(request.Input.Metadata),
		len(request.Input.Title),
		len(request.Input.Description),
		hasRepoURL,
		safeRepoURL,
	)
}

func logOverviewSnapshot(logger usecase.Logger, source string, action string, request usecase.OverviewRequest) {
	trimmedAction := strings.TrimSpace(action)
	repo := request.Input.Target.Repository
	number := request.Input.Target.ChangeRequestNumber
	if trimmedAction == "" {
		logger.Infof(
			"Pre-usecase input snapshot source=%q repository=%q changeRequestNumber=%d issueCandidates=%d.",
			strings.TrimSpace(source),
			repo,
			number,
			len(request.IssueAlignment.Candidates),
		)
	} else {
		logger.Infof(
			"Pre-usecase input snapshot source=%q action=%q repository=%q changeRequestNumber=%d issueCandidates=%d.",
			strings.TrimSpace(source),
			trimmedAction,
			repo,
			number,
			len(request.IssueAlignment.Candidates),
		)
	}

	safeRepoURL, hasRepoURL := sanitizeRepoURL(request.Input.RepoURL)
	logger.Debugf(
		"Pre-usecase input details source=%q action=%q base=%q head=%q metadataKeys=%q metadataCount=%d titleLength=%d descriptionLength=%d repoURLPresent=%t repoURLSafe=%q.",
		strings.TrimSpace(source),
		trimmedAction,
		request.Input.Base,
		request.Input.Head,
		strings.Join(sortedMetadataKeys(request.Input.Metadata), ","),
		len(request.Input.Metadata),
		len(request.Input.Title),
		len(request.Input.Description),
		hasRepoURL,
		safeRepoURL,
	)
}

func logAutogenSnapshot(logger usecase.Logger, source string, action string, request usecase.AutogenRequest) {
	trimmedAction := strings.TrimSpace(action)
	repo := request.Input.Target.Repository
	number := request.Input.Target.ChangeRequestNumber
	if trimmedAction == "" {
		logger.Infof(
			"Pre-usecase input snapshot source=%q repository=%q changeRequestNumber=%d docs=%t tests=%t publish=%t.",
			strings.TrimSpace(source),
			repo,
			number,
			request.Docs,
			request.Tests,
			request.Publish,
		)
	} else {
		logger.Infof(
			"Pre-usecase input snapshot source=%q action=%q repository=%q changeRequestNumber=%d docs=%t tests=%t publish=%t.",
			strings.TrimSpace(source),
			trimmedAction,
			repo,
			number,
			request.Docs,
			request.Tests,
			request.Publish,
		)
	}

	safeRepoURL, hasRepoURL := sanitizeRepoURL(request.Input.RepoURL)
	logger.Debugf(
		"Pre-usecase input details source=%q action=%q base=%q head=%q metadataKeys=%q metadataCount=%d titleLength=%d descriptionLength=%d repoURLPresent=%t repoURLSafe=%q.",
		strings.TrimSpace(source),
		trimmedAction,
		request.Input.Base,
		request.Input.Head,
		strings.Join(sortedMetadataKeys(request.Input.Metadata), ","),
		len(request.Input.Metadata),
		len(request.Input.Title),
		len(request.Input.Description),
		hasRepoURL,
		safeRepoURL,
	)
}

func logReplyCommentSnapshot(logger usecase.Logger, source string, action string, request usecase.ReplyCommentRequest) {
	trimmedAction := strings.TrimSpace(action)
	if trimmedAction == "" {
		logger.Infof(
			"Pre-usecase input snapshot source=%q repository=%q changeRequestNumber=%d commentID=%d commentKind=%q publish=%t.",
			strings.TrimSpace(source),
			request.Repository,
			request.ChangeRequestNumber,
			request.CommentID,
			request.CommentKind,
			request.Publish,
		)
	} else {
		logger.Infof(
			"Pre-usecase input snapshot source=%q action=%q repository=%q changeRequestNumber=%d commentID=%d commentKind=%q publish=%t.",
			strings.TrimSpace(source),
			trimmedAction,
			request.Repository,
			request.ChangeRequestNumber,
			request.CommentID,
			request.CommentKind,
			request.Publish,
		)
	}

	logger.Debugf(
		"Pre-usecase input details source=%q action=%q base=%q head=%q metadataKeys=%q metadataCount=%d titleLength=%d descriptionLength=%d questionLength=%d threadComments=%d.",
		strings.TrimSpace(source),
		trimmedAction,
		request.Base,
		request.Head,
		strings.Join(sortedMetadataKeys(request.Metadata), ","),
		len(request.Metadata),
		len(request.Title),
		len(request.Description),
		len(request.Question),
		len(request.Thread.Comments),
	)
}

func ensureLogger(logger usecase.Logger) usecase.Logger {
	if logger == nil {
		return stdlogger.Nop()
	}
	return logger
}

func sortedMetadataKeys(metadata map[string]string) []string {
	if len(metadata) == 0 {
		return nil
	}

	keys := make([]string, 0, len(metadata))
	for key := range metadata {
		trimmed := strings.TrimSpace(key)
		if trimmed == "" {
			continue
		}
		keys = append(keys, trimmed)
	}
	sort.Strings(keys)
	return keys
}

func sanitizeRepoURL(rawRepoURL string) (string, bool) {
	trimmed := strings.TrimSpace(rawRepoURL)
	if trimmed == "" {
		return "", false
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "present", true
	}

	if parsed.Scheme == "" || parsed.Host == "" {
		return "present", true
	}

	parsed.User = nil
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), true
}
