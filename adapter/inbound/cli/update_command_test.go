package cli

import (
	"context"
	"errors"
	"testing"

	"bentos-backend/shared/toolinstall"

	"github.com/stretchr/testify/require"
)

type updateTestUpdater struct {
	result toolinstall.UpdateResult
	err    error
	called bool
}

func (u *updateTestUpdater) Update(context.Context) (toolinstall.UpdateResult, error) {
	u.called = true
	return u.result, u.err
}

func TestUpdateCommandRunsUpdater(t *testing.T) {
	updater := &updateTestUpdater{result: toolinstall.UpdateResult{Version: "v1.2.3"}}
	cmd := &UpdateCommand{updater: updater}

	result, err := cmd.Run(context.Background())
	require.NoError(t, err)
	require.True(t, updater.called)
	require.Equal(t, "v1.2.3", result.Version)
}

func TestUpdateCommandReturnsError(t *testing.T) {
	updater := &updateTestUpdater{err: errors.New("boom")}
	cmd := &UpdateCommand{updater: updater}

	_, err := cmd.Run(context.Background())
	require.Error(t, err)
	require.True(t, updater.called)
}
