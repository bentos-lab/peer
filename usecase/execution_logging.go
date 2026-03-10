package usecase

import (
	"time"

	"bentos-backend/domain"
)

func logExecutionStarted(logger Logger, flow string, target domain.ChangeRequestTarget) {
	logger.Infof("%s execution started.", title(flow))
	logger.Debugf("%s target is repository %q with change request number %d.", title(flow), target.Repository, target.ChangeRequestNumber)
}

func logExecutionCompleted(logger Logger, flow string, target domain.ChangeRequestTarget, startedAt time.Time, detailFormat string, detailArgs ...any) {
	logger.Infof("%s execution completed.", title(flow))
	logger.Debugf("%s target was repository %q with change request number %d.", title(flow), target.Repository, target.ChangeRequestNumber)
	if detailFormat != "" {
		logger.Debugf(detailFormat, detailArgs...)
	}
}

func logStageSuccess(logger Logger, flow string, stage string, target domain.ChangeRequestTarget, startedAt time.Time) {
	logger.Debugf("Stage %q finished for repository %q and change request %d.", stage, target.Repository, target.ChangeRequestNumber)
	logger.Debugf("%s stage %q took %d ms.", title(flow), stage, time.Since(startedAt).Milliseconds())
}

func logStageFailure(logger Logger, flow string, stage string, target domain.ChangeRequestTarget, startedAt time.Time, err error) {
	logger.Errorf("%s stage failed.", title(flow))
	logger.Debugf("Stage %q failed for repository %q and change request number %d.", stage, target.Repository, target.ChangeRequestNumber)
	logger.Debugf("The failed stage ran for %d ms.", time.Since(startedAt).Milliseconds())
	logger.Debugf("Failure details: %v.", err)
}

func title(flow string) string {
	switch flow {
	case "review":
		return "Review"
	case "overview":
		return "Overview"
	case "change request":
		return "Change request"
	default:
		return "Usecase"
	}
}
