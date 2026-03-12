package logging

import (
	"strings"

	"bentos-backend/shared/logger/stdlogger"
	"bentos-backend/usecase"
)

// LogReplyCommentInputSnapshot logs a safe summary of replycomment parameters before usecase execution.
func LogReplyCommentInputSnapshot(logger usecase.Logger, source string, action string, request usecase.ReplyCommentRequest) {
	if logger == nil {
		logger = stdlogger.Nop()
	}
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
