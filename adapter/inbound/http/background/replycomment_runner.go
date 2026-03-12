package background

import (
	"context"
	"time"

	"bentos-backend/shared/logger/stdlogger"
	"bentos-backend/usecase"
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
	if logger == nil {
		logger = stdlogger.Nop()
	}
	if decorateContext == nil {
		decorateContext = func(ctx context.Context) context.Context { return ctx }
	}

	go func() {
		startedAt := time.Now()
		logger.Infof("%s webhook background replycomment started.", providerName)
		logger.Debugf("Webhook context repo=%q pr=%d action=%q.", request.Repository, request.ChangeRequestNumber, action)

		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Errorf(
					"%s webhook background replycomment panicked for %q#%d action=%q after %d ms: %v.",
					providerName,
					request.Repository,
					request.ChangeRequestNumber,
					action,
					time.Since(startedAt).Milliseconds(),
					recovered,
				)
			}
		}()

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		ctx = decorateContext(ctx)

		if err := execute(ctx, request); err != nil {
			logger.Debugf(
				"%s webhook background replycomment failed for %q#%d action=%q after %d ms.",
				providerName,
				request.Repository,
				request.ChangeRequestNumber,
				action,
				time.Since(startedAt).Milliseconds(),
			)
			return
		}

		logger.Infof(
			"%s webhook background replycomment completed for %q#%d action=%q in %d ms.",
			providerName,
			request.Repository,
			request.ChangeRequestNumber,
			action,
			time.Since(startedAt).Milliseconds(),
		)
	}()
}
