package github

import (
	"context"
	"errors"
	"testing"

	"bentos-backend/domain"
	"bentos-backend/usecase"
	"github.com/stretchr/testify/require"
)

type fakeOverviewClient struct {
	bodies []string
	err    error
}

func (f *fakeOverviewClient) CreateComment(_ context.Context, _ string, _ int, body string) error {
	if f.err != nil {
		return f.err
	}
	f.bodies = append(f.bodies, body)
	return nil
}

func TestOverviewPublisher_PublishOverview_PostsMarkdown(t *testing.T) {
	client := &fakeOverviewClient{}
	logger := &spyLogger{}
	publisher := NewOverviewPublisher(client, logger)

	err := publisher.PublishOverview(context.Background(), usecase.OverviewPublishRequest{
		Target: domain.ChangeRequestTarget{Repository: "org/repo", ChangeRequestNumber: 11},
		Overview: usecase.LLMOverviewResult{
			Categories: []domain.OverviewCategoryItem{
				{Category: domain.OverviewCategoryLogicUpdates, Summary: "Updated request handling."},
			},
			Walkthroughs: []domain.OverviewWalkthrough{
				{GroupName: "Handlers", Files: []string{"a.go", "b.go"}, Summary: "Moved validation and routing logic."},
			},
		},
		Metadata: map[string]string{"action": "opened"},
	})
	require.NoError(t, err)
	require.Len(t, client.bodies, 1)
	require.Contains(t, client.bodies[0], "## Summary")
	require.Contains(t, client.bodies[0], "## Walkthroughs")
	require.Contains(t, client.bodies[0], "| Group | Summary |")
	require.True(t, containsEvent(logger.events, "debug:GitHub overview comment metadata state=\"success\""))
	require.True(t, containsEvent(logger.events, "trace:GitHub overview comment content state=\"success\""))
}

func TestOverviewPublisher_PublishOverview_SkipsNonOpenedAction(t *testing.T) {
	client := &fakeOverviewClient{}
	publisher := NewOverviewPublisher(client, nil)

	err := publisher.PublishOverview(context.Background(), usecase.OverviewPublishRequest{
		Target:   domain.ChangeRequestTarget{Repository: "org/repo", ChangeRequestNumber: 11},
		Overview: usecase.LLMOverviewResult{},
		Metadata: map[string]string{"action": "synchronize"},
	})
	require.NoError(t, err)
	require.Empty(t, client.bodies)
}

func TestOverviewPublisher_PublishOverview_FailsWhenClientFails(t *testing.T) {
	client := &fakeOverviewClient{err: errors.New("network")}
	logger := &spyLogger{}
	publisher := NewOverviewPublisher(client, logger)

	err := publisher.PublishOverview(context.Background(), usecase.OverviewPublishRequest{
		Target: domain.ChangeRequestTarget{Repository: "org/repo", ChangeRequestNumber: 11},
		Overview: usecase.LLMOverviewResult{
			Walkthroughs: []domain.OverviewWalkthrough{{GroupName: "A", Summary: "B"}},
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "network")
	require.True(t, containsEvent(logger.events, "debug:GitHub overview comment metadata state=\"failed\""))
	require.True(t, containsEvent(logger.events, "trace:GitHub overview comment content state=\"failed\""))
}
