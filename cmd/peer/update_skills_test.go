package main

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"bentos-backend/shared/skillupdate"

	"github.com/stretchr/testify/require"
)

type stubSkillUpdateRunner struct {
	results []skillupdate.Result
	err     error
	paths   []string
}

func (s *stubSkillUpdateRunner) Run(_ context.Context, paths []string) ([]skillupdate.Result, error) {
	s.paths = append([]string{}, paths...)
	return s.results, s.err
}

func TestUpdateSkillsSubcommandOutput(t *testing.T) {
	runner := &stubSkillUpdateRunner{
		results: []skillupdate.Result{
			{Path: "/tmp/skills/peer", Err: nil},
			{Path: "/tmp/skills2/peer", Err: errors.New("boom")},
		},
		err: errors.New("combined"),
	}
	cmd := newUpdateSkillsSubcommand(context.Background(), runner)
	buffer := &bytes.Buffer{}
	cmd.SetOut(buffer)
	cmd.SetErr(buffer)
	cmd.SetArgs([]string{"--path", "/tmp/skills", "--path", "/tmp/skills2"})

	err := cmd.Execute()
	require.Error(t, err)
	require.Equal(t, []string{"/tmp/skills", "/tmp/skills2"}, runner.paths)

	output := buffer.String()
	require.Contains(t, output, "/tmp/skills/peer ok")
	require.Contains(t, output, "/tmp/skills2/peer error: boom")
}
