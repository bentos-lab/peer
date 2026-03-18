package cli

import (
	"context"
	"errors"
	"testing"

	"github.com/bentos-lab/peer/shared/toolinstall"

	"github.com/stretchr/testify/require"
)

type updateTestUpdater struct {
	result       toolinstall.UpdateResult
	err          error
	latest       string
	latestErr    error
	updateCalled bool
	latestCalled bool
}

func (u *updateTestUpdater) LatestVersion(context.Context) (string, error) {
	u.latestCalled = true
	return u.latest, u.latestErr
}

func (u *updateTestUpdater) Update(context.Context) (toolinstall.UpdateResult, error) {
	u.updateCalled = true
	return u.result, u.err
}

func TestUpdateCommandRunsUpdater(t *testing.T) {
	updater := &updateTestUpdater{
		latest: "v9.9.9",
		result: toolinstall.UpdateResult{Version: "v1.2.3"},
	}
	cmd := &UpdateCommand{updater: updater, currentVersion: "v1.0.0"}

	result, err := cmd.Run(context.Background())
	require.NoError(t, err)
	require.True(t, updater.latestCalled)
	require.True(t, updater.updateCalled)
	require.Equal(t, "v1.2.3", result.Result.Version)
	require.False(t, result.UpToDate)
}

func TestUpdateCommandReturnsError(t *testing.T) {
	updater := &updateTestUpdater{
		latest: "v1.2.3",
		err:    errors.New("boom"),
	}
	cmd := &UpdateCommand{updater: updater, currentVersion: "v1.0.0"}

	_, err := cmd.Run(context.Background())
	require.Error(t, err)
	require.True(t, updater.latestCalled)
	require.True(t, updater.updateCalled)
}

func TestUpdateCommandSkipsWhenUpToDate(t *testing.T) {
	updater := &updateTestUpdater{latest: "v1.2.3"}
	cmd := &UpdateCommand{updater: updater, currentVersion: "1.2.3"}

	result, err := cmd.Run(context.Background())
	require.NoError(t, err)
	require.True(t, updater.latestCalled)
	require.False(t, updater.updateCalled)
	require.True(t, result.UpToDate)
	require.Equal(t, "v1.2.3", result.Result.Version)
}

func TestUpdateCommandIgnoresMetadataSuffix(t *testing.T) {
	updater := &updateTestUpdater{latest: "v1.2.3+build.1"}
	cmd := &UpdateCommand{updater: updater, currentVersion: "1.2.3"}

	result, err := cmd.Run(context.Background())
	require.NoError(t, err)
	require.True(t, updater.latestCalled)
	require.False(t, updater.updateCalled)
	require.True(t, result.UpToDate)
	require.Equal(t, "v1.2.3+build.1", result.Result.Version)
}
