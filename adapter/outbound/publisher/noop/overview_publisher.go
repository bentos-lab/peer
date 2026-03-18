package noop

import (
	"context"

	"github.com/bentos-lab/peer/usecase"
)

type overviewPublisher struct{}

// NewOverviewPublisher creates an overview publisher that ignores publish requests.
func NewOverviewPublisher() usecase.OverviewPublisher {
	return &overviewPublisher{}
}

// PublishOverview ignores publish requests and always succeeds.
func (*overviewPublisher) PublishOverview(_ context.Context, _ usecase.OverviewPublishRequest) error {
	return nil
}
