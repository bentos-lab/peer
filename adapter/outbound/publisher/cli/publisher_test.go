package cli

import (
	"bytes"
	"context"
	"testing"

	"bentos-backend/domain"
	"bentos-backend/usecase"
	"github.com/stretchr/testify/require"
)

func TestPublisher_Publish(t *testing.T) {
	var buffer bytes.Buffer
	pub := NewPublisher(&buffer)
	err := pub.Publish(context.Background(), usecase.ReviewPublishResult{
		Messages: []domain.ReviewMessage{
			{
				Type:  domain.ReviewMessageTypeFileGroup,
				Title: "Review notes for `a.go`",
				Body:  "- [MAJOR] test",
			},
			{
				Type:  domain.ReviewMessageTypeSummary,
				Title: "Review summary",
				Body:  "Found 1 issue.",
			},
		},
	})
	require.NoError(t, err)
	require.Contains(t, buffer.String(), "Review notes for `a.go`")
	require.Contains(t, buffer.String(), "Review summary")
}
