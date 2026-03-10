package github

import (
	"context"
	"testing"

	"bentos-backend/domain"
	"bentos-backend/usecase"
	uccontracts "bentos-backend/usecase/contracts"
	"github.com/stretchr/testify/require"
)

type autogenTestCommentClient struct {
	bodies []string
}

func (c *autogenTestCommentClient) CreateComment(_ context.Context, _ string, _ int, body string) error {
	c.bodies = append(c.bodies, body)
	return nil
}

type autogenTestEnvironment struct {
	lastOptions domain.CodeEnvironmentPushOptions
	pushCalls   int
}

func (e *autogenTestEnvironment) SetupAgent(_ context.Context, _ domain.CodingAgentSetupOptions) (uccontracts.CodingAgent, error) {
	return nil, nil
}

func (e *autogenTestEnvironment) LoadChangedFiles(_ context.Context, _ domain.CodeEnvironmentLoadOptions) ([]domain.ChangedFile, error) {
	return nil, nil
}

func (e *autogenTestEnvironment) PushChanges(_ context.Context, opts domain.CodeEnvironmentPushOptions) (domain.CodeEnvironmentPushResult, error) {
	e.pushCalls++
	e.lastOptions = opts
	return domain.CodeEnvironmentPushResult{Pushed: true}, nil
}

func (e *autogenTestEnvironment) Cleanup(_ context.Context) error {
	return nil
}

func TestAutogenPublisherPushesWithEnvironment(t *testing.T) {
	env := &autogenTestEnvironment{}

	client := &autogenTestCommentClient{}
	publisher := NewAutogenPublisher(client, nil)

	err := publisher.PublishAutogen(context.Background(), usecase.AutogenPublishRequest{
		Target:      domain.ChangeRequestTarget{Repository: "org/repo", ChangeRequestNumber: 9},
		Publish:     true,
		HeadBranch:  "feature",
		Changes:     []domain.AutogenChange{{FilePath: "foo.go", StartLine: 1, EndLine: 1, Content: "// add"}},
		Summary:     domain.AutogenSummary{},
		AgentOutput: "Autogen agent report",
		Environment: env,
		PushOptions: domain.CodeEnvironmentPushOptions{
			TargetBranch:  "feature",
			CommitMessage: "autogen: add tests/docs/comments",
			RemoteName:    "origin",
		},
	})
	require.NoError(t, err)
	require.Len(t, client.bodies, 1)
	require.Contains(t, client.bodies[0], "Agent output")
	require.Equal(t, 1, env.pushCalls)
	require.Equal(t, "feature", env.lastOptions.TargetBranch)
}

func TestAutogenPublisherSkipsWhenNoChanges(t *testing.T) {
	env := &autogenTestEnvironment{}

	client := &autogenTestCommentClient{}
	logger := &spyLogger{}
	publisher := NewAutogenPublisher(client, logger)

	err := publisher.PublishAutogen(context.Background(), usecase.AutogenPublishRequest{
		Target:      domain.ChangeRequestTarget{Repository: "org/repo", ChangeRequestNumber: 9},
		Publish:     true,
		HeadBranch:  "feature",
		Summary:     domain.AutogenSummary{},
		AgentOutput: "Autogen agent report",
		Environment: env,
		PushOptions: domain.CodeEnvironmentPushOptions{
			TargetBranch:  "feature",
			CommitMessage: "autogen: add tests/docs/comments",
			RemoteName:    "origin",
		},
	})
	require.NoError(t, err)
	require.Empty(t, client.bodies)
	require.Equal(t, 0, env.pushCalls)
	require.True(t, containsEvent(logger.events, "No autogen docs/tests/comments added; skipping publish."))
}
