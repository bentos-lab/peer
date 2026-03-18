package noop

import (
	"context"
	"testing"

	"github.com/bentos-lab/peer/usecase"
	"github.com/stretchr/testify/require"
)

func TestOverviewPublisherPublishOverviewReturnsNil(t *testing.T) {
	publisher := NewOverviewPublisher()

	err := publisher.PublishOverview(context.Background(), usecase.OverviewPublishRequest{})

	require.NoError(t, err)
}
