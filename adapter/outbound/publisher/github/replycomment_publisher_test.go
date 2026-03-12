package github

import (
	"context"
	"testing"

	"bentos-backend/domain"
	"bentos-backend/usecase"
	"github.com/stretchr/testify/require"
)

type replycommentTestClient struct {
	bodies []string
}

func (c *replycommentTestClient) CreateComment(_ context.Context, _ string, _ int, body string) error {
	c.bodies = append(c.bodies, body)
	return nil
}

func (c *replycommentTestClient) CreateReviewReply(_ context.Context, _ string, _ int, _ int64, body string) error {
	c.bodies = append(c.bodies, body)
	return nil
}

func TestReplyCommentPublisherPrependsRecipeWarning(t *testing.T) {
	client := &replycommentTestClient{}
	publisher := NewReplyCommentPublisher(client, nil)

	err := publisher.Publish(context.Background(), usecase.ReplyCommentPublishResult{
		Target: domain.ChangeRequestTarget{
			Repository:          "org/repo",
			ChangeRequestNumber: 9,
		},
		CommentID:      123,
		Kind:           domain.CommentKindIssue,
		Body:           "Answer body",
		ShouldPost:     true,
		RecipeWarnings: []string{".autogit/reply.md"},
	})
	require.NoError(t, err)
	require.Len(t, client.bodies, 1)
	require.Contains(t, client.bodies[0], ".autogit/reply.md")
}
