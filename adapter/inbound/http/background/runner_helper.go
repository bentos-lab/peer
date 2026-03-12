package background

import (
	"context"
	"time"

	"bentos-backend/shared/logger/stdlogger"
	"bentos-backend/usecase"
)

type requestInfoFunc[T any] func(T) (string, int)

type snapshotLogger[T any] func(usecase.Logger, string, string, T)

type requestExecutor[T any] func(context.Context, T) error

func runAsync[T any](
	logger usecase.Logger,
	providerName string,
	subject string,
	action string,
	request T,
	timeout time.Duration,
	decorateContext func(context.Context) context.Context,
	logSnapshot snapshotLogger[T],
	execute requestExecutor[T],
	requestInfo requestInfoFunc[T],
) {
	if logger == nil {
		logger = stdlogger.Nop()
	}
	if decorateContext == nil {
		decorateContext = func(ctx context.Context) context.Context { return ctx }
	}

	repository, prNumber := requestInfo(request)

	go func() {
		startedAt := time.Now()
		logger.Infof("%s webhook background %s started.", providerName, subject)
		logger.Debugf("Webhook context repo=%q pr=%d action=%q.", repository, prNumber, action)

		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Errorf(
					"%s webhook background %s panicked for %q#%d action=%q after %d ms: %v.",
					providerName,
					subject,
					repository,
					prNumber,
					action,
					time.Since(startedAt).Milliseconds(),
					recovered,
				)
			}
		}()

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		ctx = decorateContext(ctx)
		if logSnapshot != nil {
			logSnapshot(logger, "webhook", action, request)
		}

		if err := execute(ctx, request); err != nil {
			logger.Debugf(
				"%s webhook background %s failed for %q#%d action=%q after %d ms.",
				providerName,
				subject,
				repository,
				prNumber,
				action,
				time.Since(startedAt).Milliseconds(),
			)
			return
		}

		logger.Infof(
			"%s webhook background %s completed for %q#%d action=%q in %d ms.",
			providerName,
			subject,
			repository,
			prNumber,
			action,
			time.Since(startedAt).Milliseconds(),
		)
	}()
}
