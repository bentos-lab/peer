package gitlab

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	gitlabvcs "github.com/bentos-lab/peer/adapter/outbound/vcs/gitlab"
	"github.com/stretchr/testify/require"
)

type fakeHookClient struct {
	events         []gitlabvcs.UserEvent
	maintainer     bool
	repo           string
	hooks          []gitlabvcs.ProjectHook
	createdHooks   []gitlabvcs.HookInput
	updatedHooks   []gitlabvcs.HookInput
	updatedHookIDs []int
}

func (c *fakeHookClient) ListUserEvents(_ context.Context, _ time.Time) ([]gitlabvcs.UserEvent, error) {
	return c.events, nil
}

func (c *fakeHookClient) HasMaintainerAccess(_ context.Context, _ int) (bool, string, error) {
	return c.maintainer, c.repo, nil
}

func (c *fakeHookClient) ListProjectHooks(_ context.Context, _ int) ([]gitlabvcs.ProjectHook, error) {
	return c.hooks, nil
}

func (c *fakeHookClient) CreateProjectHook(_ context.Context, _ int, input gitlabvcs.HookInput) error {
	c.createdHooks = append(c.createdHooks, input)
	return nil
}

func (c *fakeHookClient) UpdateProjectHook(_ context.Context, _ int, hookID int, input gitlabvcs.HookInput) error {
	c.updatedHookIDs = append(c.updatedHookIDs, hookID)
	c.updatedHooks = append(c.updatedHooks, input)
	return nil
}

type noopLogger struct{}

func (noopLogger) Debugf(string, ...any) {}
func (noopLogger) Infof(string, ...any)  {}
func (noopLogger) Warnf(string, ...any)  {}
func (noopLogger) Errorf(string, ...any) {}

func TestHookSyncerCreatesHookWhenMissing(t *testing.T) {
	client := &fakeHookClient{
		events:     []gitlabvcs.UserEvent{{ProjectID: 1, CreatedAt: time.Now().UTC().Add(time.Minute)}},
		maintainer: true,
		repo:       "group/repo",
		hooks:      nil,
	}
	statePath := filepath.Join(t.TempDir(), "state.json")
	syncer := NewHookSyncer(client, noopLogger{}, "https://hooks.example.com/webhook/gitlab", "secret", time.Minute, statePath)
	syncer.runOnce(context.Background())

	require.Len(t, client.createdHooks, 1)
	require.Equal(t, "https://hooks.example.com/webhook/gitlab", client.createdHooks[0].URL)
	require.True(t, client.createdHooks[0].MergeRequestsEvents)
	require.True(t, client.createdHooks[0].NoteEvents)
}

func TestHookSyncerUpdatesHookWhenMismatch(t *testing.T) {
	client := &fakeHookClient{
		events:     []gitlabvcs.UserEvent{{ProjectID: 2, CreatedAt: time.Now().UTC().Add(time.Minute)}},
		maintainer: true,
		repo:       "group/repo",
		hooks: []gitlabvcs.ProjectHook{{
			ID:                  9,
			URL:                 "https://hooks.example.com/webhook/gitlab",
			Token:               "old",
			MergeRequestsEvents: true,
			NoteEvents:          false,
		}},
	}
	statePath := filepath.Join(t.TempDir(), "state.json")
	syncer := NewHookSyncer(client, noopLogger{}, "https://hooks.example.com/webhook/gitlab", "secret", time.Minute, statePath)
	syncer.runOnce(context.Background())

	require.Len(t, client.updatedHooks, 1)
	require.Equal(t, 9, client.updatedHookIDs[0])
	require.Equal(t, "secret", client.updatedHooks[0].Token)
	require.True(t, client.updatedHooks[0].NoteEvents)
}

func TestHookSyncerSkipsWithoutAccess(t *testing.T) {
	client := &fakeHookClient{
		events:     []gitlabvcs.UserEvent{{ProjectID: 3, CreatedAt: time.Now().UTC().Add(time.Minute)}},
		maintainer: false,
		repo:       "group/repo",
		hooks:      nil,
	}
	statePath := filepath.Join(t.TempDir(), "state.json")
	syncer := NewHookSyncer(client, noopLogger{}, "https://hooks.example.com/webhook/gitlab", "secret", time.Minute, statePath)
	syncer.runOnce(context.Background())

	require.Len(t, client.createdHooks, 0)
	require.Len(t, client.updatedHooks, 0)
}
