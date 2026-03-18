package background

import (
	"context"
	"time"

	sharedlogging "github.com/bentos-lab/peer/shared/logging"
	"github.com/bentos-lab/peer/usecase"
)

// RunReplyCommentAsync executes one replycomment request in a background goroutine with standardized logging.
func RunReplyCommentAsync(
	logger usecase.Logger,
	providerName string,
	action string,
	request usecase.ReplyCommentRequest,
	timeout time.Duration,
	decorateContext func(context.Context) context.Context,
	execute func(context.Context, usecase.ReplyCommentRequest) error,
) {
	runAsync(
		logger,
		providerName,
		"replycomment",
		action,
		request,
		timeout,
		decorateContext,
		func(log usecase.Logger, source string, action string, req usecase.ReplyCommentRequest) {
			sharedlogging.LogInputSnapshot(log, source, action, req)
		},
		execute,
		func(req usecase.ReplyCommentRequest) (string, int) {
			return req.Repository, req.ChangeRequestNumber
		},
	)
}
