package usecase

import (
	"fmt"
	"time"

	"github.com/bentos-lab/peer/domain"
)

func logExecution(logger Logger, flow string, target domain.ChangeRequestTarget, status string, startedAt time.Time, detailFormat string, detailArgs ...any) {
	switch status {
	case "start":
		logger.Infof("%s execution started for %q#%d.", title(flow), target.Repository, target.ChangeRequestNumber)
	case "complete":
		logger.Infof(
			"%s execution completed for %q#%d in %d ms.",
			title(flow),
			target.Repository,
			target.ChangeRequestNumber,
			time.Since(startedAt).Milliseconds(),
		)
	default:
		logger.Infof("%s execution status=%q for %q#%d.", title(flow), status, target.Repository, target.ChangeRequestNumber)
	}
	if detailFormat != "" {
		logger.Debugf(detailFormat, detailArgs...)
	}
}

func logStage(
	logger Logger,
	flow string,
	stage string,
	target domain.ChangeRequestTarget,
	outcome string,
	startedAt time.Time,
	detailFormat string,
	detailArgs ...any,
) {
	elapsedMs := time.Since(startedAt).Milliseconds()
	switch outcome {
	case "success":
		logger.Debugf(
			"%s stage %q completed for %q#%d in %d ms.",
			title(flow),
			stage,
			target.Repository,
			target.ChangeRequestNumber,
			elapsedMs,
		)
	case "failure":
		err := fmt.Errorf(detailFormat, detailArgs...)
		logger.Errorf(
			"%s stage %q failed for %q#%d after %d ms: %v.",
			title(flow),
			stage,
			target.Repository,
			target.ChangeRequestNumber,
			elapsedMs,
			err,
		)
	default:
		logger.Debugf(
			"%s stage %q outcome=%q for %q#%d in %d ms.",
			title(flow),
			stage,
			outcome,
			target.Repository,
			target.ChangeRequestNumber,
			elapsedMs,
		)
	}
}

func title(flow string) string {
	switch flow {
	case "review":
		return "Review"
	case "overview":
		return "Overview"
	case "autogen":
		return "Autogen"
	case "commit":
		return "Commit"
	case "change request":
		return "Change request"
	case "replycomment":
		return "Reply comment"
	default:
		return "Usecase"
	}
}
