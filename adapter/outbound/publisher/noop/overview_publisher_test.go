package noop

import (
	"context"
	"testing"

	"bentos-backend/usecase"
	"github.com/stretchr/testify/require"
)

func TestOverviewPublisherPublishOverviewReturnsNil(t *testing.T) {
	publisher := NewOverviewPublisher()

	err := publisher.PublishOverview(context.Background(), usecase.OverviewPublishRequest{})

	require.NoError(t, err)
}
