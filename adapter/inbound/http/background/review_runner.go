package background

import (
	"context"
	"time"

	sharedlogging "bentos-backend/shared/logging"
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
	runAsync(
		logger,
		providerName,
		"review",
		action,
		request,
		timeout,
		decorateContext,
		func(log usecase.Logger, source string, action string, req usecase.ChangeRequestRequest) {
			sharedlogging.LogInputSnapshot(log, source, action, req)
		},
		execute,
		func(req usecase.ChangeRequestRequest) (string, int) {
			return req.Repository, req.ChangeRequestNumber
		},
	)
}
