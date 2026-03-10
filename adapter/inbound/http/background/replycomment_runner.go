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
		logger.Debugf("Repository is %q and change request number is %d.", request.Repository, request.ChangeRequestNumber)
		logger.Debugf("Webhook action is %q.", action)

		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Errorf("%s webhook background replycomment panicked.", providerName)
				logger.Debugf("Repository is %q, change request number is %d, and webhook action is %q.", request.Repository, request.ChangeRequestNumber, action)
				logger.Debugf("The background replycomment ran for %d ms before panicking.", time.Since(startedAt).Milliseconds())
				logger.Debugf("Panic details: %v.", recovered)
			}
		}()

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		ctx = decorateContext(ctx)

		if err := execute(ctx, request); err != nil {
			logger.Errorf("%s webhook background replycomment failed.", providerName)
			logger.Debugf("Repository is %q, change request number is %d, and webhook action is %q.", request.Repository, request.ChangeRequestNumber, action)
			logger.Debugf("The background replycomment ran for %d ms before failing.", time.Since(startedAt).Milliseconds())
			logger.Debugf("Failure details: %v.", err)
			return
		}

		logger.Infof("%s webhook background replycomment completed.", providerName)
		logger.Debugf("Repository is %q, change request number is %d, and webhook action is %q.", request.Repository, request.ChangeRequestNumber, action)
		logger.Debugf("The background replycomment completed in %d ms.", time.Since(startedAt).Milliseconds())
	}()
}
