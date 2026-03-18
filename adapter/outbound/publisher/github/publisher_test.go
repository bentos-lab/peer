package github

import (
	"context"
	"testing"

	"bentos-backend/domain"
	"bentos-backend/usecase"
	"github.com/stretchr/testify/require"
)

type fakeClient struct {
	bodies []string
}

func (f *fakeClient) CreateComment(_ context.Context, _ string, _ int, body string) error {
	f.bodies = append(f.bodies, body)
	return nil
}

func TestPublisher_Publish(t *testing.T) {
	client := &fakeClient{}
	pub := NewPublisher(client)
	err := pub.Publish(context.Background(), usecase.ReviewPublishResult{
		Target: domain.ReviewTarget{
			Repository:          "org/repo",
			ChangeRequestNumber: 11,
		},
		Messages: []domain.ReviewMessage{
			{Title: "A", Body: "B"},
			{Title: "Summary", Body: "Done"},
		},
	})
	require.NoError(t, err)
	require.Len(t, client.bodies, 2)
	require.Contains(t, client.bodies[0], "A")
}
