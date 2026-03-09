package background

import (
	"context"
	"time"

	inboundlogging "bentos-backend/adapter/inbound/logging"
	"bentos-backend/shared/logger/stdlogger"
	"bentos-backend/usecase"
)

// RunReviewAsync executes one review request in a background goroutine with standardized logging.
func RunReviewAsync(
	logger usecase.Logger,
	providerName string,
	action string,
	request usecase.ChangeRequestRequest,
	timeout time.Duration,
	decorateContext func(context.Context) context.Context,
	execute func(context.Context, usecase.ChangeRequestRequest) error,
) {
	if logger == nil {
		logger = stdlogger.Nop()
	}
	if decorateContext == nil {
		decorateContext = func(ctx context.Context) context.Context { return ctx }
	}

	go func() {
		startedAt := time.Now()
		logger.Infof("%s webhook background review started.", providerName)
		logger.Debugf("Repository is %q and change request number is %d.", request.Repository, request.ChangeRequestNumber)
		logger.Debugf("Webhook action is %q.", action)

		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Errorf("%s webhook background review panicked.", providerName)
				logger.Debugf("Repository is %q, change request number is %d, and webhook action is %q.", request.Repository, request.ChangeRequestNumber, action)
				logger.Debugf("The background review ran for %d ms before panicking.", time.Since(startedAt).Milliseconds())
				logger.Debugf("Panic details: %v.", recovered)
			}
		}()

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		ctx = decorateContext(ctx)
		inboundlogging.LogChangeRequestInputSnapshot(logger, "webhook", action, request)

		if err := execute(ctx, request); err != nil {
			logger.Errorf("%s webhook background review failed.", providerName)
			logger.Debugf("Repository is %q, change request number is %d, and webhook action is %q.", request.Repository, request.ChangeRequestNumber, action)
			logger.Debugf("The background review ran for %d ms before failing.", time.Since(startedAt).Milliseconds())
			logger.Debugf("Failure details: %v.", err)
			return
		}

		logger.Infof("%s webhook background review completed.", providerName)
		logger.Debugf("Repository is %q, change request number is %d, and webhook action is %q.", request.Repository, request.ChangeRequestNumber, action)
		logger.Debugf("The background review completed in %d ms.", time.Since(startedAt).Milliseconds())
	}()
}
