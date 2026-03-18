package cli

import (
	"context"
	"errors"
	"testing"

	"github.com/bentos-lab/peer/shared/skillupdate"

	"github.com/stretchr/testify/require"
)

type skillUpdateTestUpdater struct {
	results []skillupdate.Result
	err     error
	paths   []string
}

func (u *skillUpdateTestUpdater) Update(_ context.Context, paths []string) ([]skillupdate.Result, error) {
	u.paths = append([]string{}, paths...)
	return u.results, u.err
}

func TestSkillUpdateCommandRunsUpdater(t *testing.T) {
	updater := &skillUpdateTestUpdater{
		results: []skillupdate.Result{{Path: "/tmp/skills", Err: nil}},
	}
	cmd := &SkillUpdateCommand{updater: updater}

	results, err := cmd.Run(context.Background(), []string{"/tmp/skills"})
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, []string{"/tmp/skills"}, updater.paths)
}

func TestSkillUpdateCommandReturnsError(t *testing.T) {
	updater := &skillUpdateTestUpdater{err: errors.New("boom")}
	cmd := &SkillUpdateCommand{updater: updater}

	_, err := cmd.Run(context.Background(), []string{"/tmp/skills"})
	require.Error(t, err)
}
