package gitlab

import (
	"context"
	"errors"
	"testing"

	"bentos-backend/adapter/outbound/commandrunner"
	"bentos-backend/domain"
	"bentos-backend/shared/toolinstall"
	"github.com/stretchr/testify/require"
)

func newTestGitLabCLIClient(runner commandrunner.Runner) *CLIClient {
	return newTestGitLabCLIClientWithHost(runner, "")
}

func newTestGitLabCLIClientWithHost(runner commandrunner.Runner, host string) *CLIClient {
	preferTTY := false
	installer := toolinstall.NewGlabInstaller(&toolinstall.Deps{
		StreamRunner: runner.(commandrunner.StreamRunner),
		PreferTTY:    &preferTTY,
		LookPath: func(name string) (string, error) {
			if name == "glab" {
				return "/bin/glab", nil
			}
			return "", errors.New("not found")
		},
		IsTerminal: func() bool { return true },
	})
	return &CLIClient{runner: runner, installer: installer, host: host}
}

func TestGitLabClient_GetPullRequestInfo(t *testing.T) {
	runner := commandrunner.NewDummyCommandRunner()
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{Name: "glab", Args: []string{"auth", "status"}},
		Result:   commandrunner.Result{Stdout: []byte("ok")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{Name: "glab", Args: []string{"api", "--method", "GET", "projects/group%2Fsubgroup%2Fproject/merge_requests/7"}},
		Result: commandrunner.Result{Stdout: []byte(`{
			"title":"MR title",
			"description":"MR body",
			"source_branch":"feature",
			"diff_refs":{"base_sha":"base123","start_sha":"start123","head_sha":"head123"}
		}`)},
	})
	client := newTestGitLabCLIClient(runner)

	info, err := client.GetPullRequestInfo(context.Background(), "group/subgroup/project", 7)
	require.NoError(t, err)
	require.Equal(t, "group/subgroup/project", info.Repository)
	require.Equal(t, "MR title", info.Title)
	require.Equal(t, "MR body", info.Description)
	require.Equal(t, "base123", info.BaseRef)
	require.Equal(t, "head123", info.HeadRef)
	require.Equal(t, "start123", info.StartRef)
	require.Equal(t, "feature", info.HeadRefName)
	require.NoError(t, runner.VerifyDone())
}

func TestGitLabClient_ListIssueCommentsSkipsSystemNotes(t *testing.T) {
	runner := commandrunner.NewDummyCommandRunner()
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{Name: "glab", Args: []string{"auth", "status"}},
		Result:   commandrunner.Result{Stdout: []byte("ok")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{Name: "glab", Args: []string{"api", "--paginate", "projects/group%2Fproject/issues/12/notes"}},
		Result: commandrunner.Result{Stdout: []byte(`[
			{"id":1,"body":"system","created_at":"2024-01-01T00:00:00Z","system":true,"author":{"username":"bot"}},
			{"id":2,"body":"hello","created_at":"2024-01-02T00:00:00Z","system":false,"author":{"username":"dev"}}
		]`)},
	})
	client := newTestGitLabCLIClient(runner)

	comments, err := client.ListIssueComments(context.Background(), "group/project", 12)
	require.NoError(t, err)
	require.Len(t, comments, 1)
	require.Equal(t, int64(2), comments[0].ID)
	require.Equal(t, "hello", comments[0].Body)
	require.Equal(t, "dev", comments[0].Author.Login)
	require.NoError(t, runner.VerifyDone())
}

func TestGitLabClient_ListReviewCommentsParsesDiscussions(t *testing.T) {
	runner := commandrunner.NewDummyCommandRunner()
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{Name: "glab", Args: []string{"auth", "status"}},
		Result:   commandrunner.Result{Stdout: []byte("ok")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{Name: "glab", Args: []string{"api", "--paginate", "projects/group%2Fproject/merge_requests/5/discussions"}},
		Result: commandrunner.Result{Stdout: []byte(`[
			{"id":"disc1","notes":[
				{"id":10,"body":"note","created_at":"2024-01-01T00:00:00Z","in_reply_to_id":0,"author":{"username":"dev"},
				 "position":{"new_path":"main.go","old_path":"","new_line":5,"old_line":0}}
			]}
		]`)},
	})
	client := newTestGitLabCLIClient(runner)

	comments, err := client.ListReviewComments(context.Background(), "group/project", 5)
	require.NoError(t, err)
	require.Len(t, comments, 1)
	require.Equal(t, int64(10), comments[0].ID)
	require.Equal(t, "main.go", comments[0].Path)
	require.Equal(t, 5, comments[0].Line)
	require.Equal(t, "RIGHT", comments[0].Side)
	require.NoError(t, runner.VerifyDone())
}

func TestGitLabClient_CreateReviewCommentClassifiesInvalidAnchor(t *testing.T) {
	runner := commandrunner.NewDummyCommandRunner()
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{Name: "glab", Args: []string{"auth", "status"}},
		Result:   commandrunner.Result{Stdout: []byte("ok")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{Name: "glab", Args: []string{"api", "--method", "GET", "projects/group%2Fproject/merge_requests/3"}},
		Result: commandrunner.Result{Stdout: []byte(`{
			"title":"MR title",
			"description":"MR body",
			"source_branch":"feature",
			"diff_refs":{"base_sha":"base123","start_sha":"start123","head_sha":"head123"}
		}`)},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{Name: "glab", Args: []string{
			"api", "--method", "POST",
			"--raw-field", "body=bad",
			"--raw-field", "position[position_type]=text",
			"--raw-field", "position[base_sha]=base123",
			"--raw-field", "position[start_sha]=start123",
			"--raw-field", "position[head_sha]=head123",
			"--raw-field", "position[new_path]=main.go",
			"--raw-field", "position[old_path]=main.go",
			"--raw-field", "position[new_line]=10",
			"projects/group%2Fproject/merge_requests/3/discussions",
		}},
		Result: commandrunner.Result{Stderr: []byte("HTTP 422: position is invalid")},
		Err:    errors.New("exit status 1"),
	})
	client := newTestGitLabCLIClient(runner)

	err := client.CreateReviewComment(context.Background(), "group/project", 3, domain.ReviewCommentInput{
		Body:      "bad",
		Path:      "main.go",
		StartLine: 10,
		EndLine:   10,
	})
	require.Error(t, err)
	require.True(t, domain.IsInvalidAnchorError(err))
	require.NoError(t, runner.VerifyDone())
}

func TestGitLabClient_CreateReviewCommentUsesOldLineWhenLineSideOld(t *testing.T) {
	runner := commandrunner.NewDummyCommandRunner()
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{Name: "glab", Args: []string{"auth", "status"}},
		Result:   commandrunner.Result{Stdout: []byte("ok")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{Name: "glab", Args: []string{"api", "--method", "GET", "projects/group%2Fproject/merge_requests/3"}},
		Result: commandrunner.Result{Stdout: []byte(`{
			"title":"MR title",
			"description":"MR body",
			"source_branch":"feature",
			"diff_refs":{"base_sha":"base123","start_sha":"start123","head_sha":"head123"}
		}`)},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{Name: "glab", Args: []string{
			"api", "--method", "POST",
			"--raw-field", "body=old",
			"--raw-field", "position[position_type]=text",
			"--raw-field", "position[base_sha]=base123",
			"--raw-field", "position[start_sha]=start123",
			"--raw-field", "position[head_sha]=head123",
			"--raw-field", "position[new_path]=main.go",
			"--raw-field", "position[old_path]=main.go",
			"--raw-field", "position[old_line]=8",
			"projects/group%2Fproject/merge_requests/3/discussions",
		}},
		Result: commandrunner.Result{Stdout: []byte(`{}`)},
	})
	client := newTestGitLabCLIClient(runner)

	err := client.CreateReviewComment(context.Background(), "group/project", 3, domain.ReviewCommentInput{
		Body:      "old",
		Path:      "main.go",
		StartLine: 8,
		EndLine:   8,
		LineSide:  domain.LineSideOld,
	})
	require.NoError(t, err)
	require.NoError(t, runner.VerifyDone())
}

func TestGitLabClient_CreateReviewCommentUsesLineRangeForMultiLine(t *testing.T) {
	runner := commandrunner.NewDummyCommandRunner()
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{Name: "glab", Args: []string{"auth", "status"}},
		Result:   commandrunner.Result{Stdout: []byte("ok")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{Name: "glab", Args: []string{"api", "--method", "GET", "projects/group%2Fproject/merge_requests/3"}},
		Result: commandrunner.Result{Stdout: []byte(`{
			"title":"MR title",
			"description":"MR body",
			"source_branch":"feature",
			"diff_refs":{"base_sha":"base123","start_sha":"start123","head_sha":"head123"}
		}`)},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{Name: "glab", Args: []string{
			"api", "--method", "POST",
			"--raw-field", "body=range",
			"--raw-field", "position[position_type]=text",
			"--raw-field", "position[base_sha]=base123",
			"--raw-field", "position[start_sha]=start123",
			"--raw-field", "position[head_sha]=head123",
			"--raw-field", "position[new_path]=main.go",
			"--raw-field", "position[old_path]=main.go",
			"--raw-field", "position[line_range][start][type]=new",
			"--raw-field", "position[line_range][start][line_code]=0607f785dfa3c3861b3239f6723eb276d8056461_0_10",
			"--raw-field", "position[line_range][start][new_line]=10",
			"--raw-field", "position[line_range][end][type]=new",
			"--raw-field", "position[line_range][end][line_code]=0607f785dfa3c3861b3239f6723eb276d8056461_0_12",
			"--raw-field", "position[line_range][end][new_line]=12",
			"projects/group%2Fproject/merge_requests/3/discussions",
		}},
		Result: commandrunner.Result{Stdout: []byte(`{}`)},
	})
	client := newTestGitLabCLIClient(runner)

	err := client.CreateReviewComment(context.Background(), "group/project", 3, domain.ReviewCommentInput{
		Body:      "range",
		Path:      "main.go",
		StartLine: 10,
		EndLine:   12,
	})
	require.NoError(t, err)
	require.NoError(t, runner.VerifyDone())
}

func TestGitLabClient_UsesHostnameFlagWhenHostProvided(t *testing.T) {
	runner := commandrunner.NewDummyCommandRunner()
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{Name: "glab", Args: []string{"--hostname", "gitlab.example.com", "auth", "status"}},
		Result:   commandrunner.Result{Stdout: []byte("ok")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{Name: "glab", Args: []string{"--hostname", "gitlab.example.com", "api", "--method", "GET", "projects/group%2Fproject/merge_requests/2"}},
		Result: commandrunner.Result{Stdout: []byte(`{
			"title":"MR title",
			"description":"MR body",
			"source_branch":"feature",
			"diff_refs":{"base_sha":"base123","start_sha":"start123","head_sha":"head123"}
		}`)},
	})
	client := newTestGitLabCLIClientWithHost(runner, "gitlab.example.com")

	_, err := client.GetPullRequestInfo(context.Background(), "group/project", 2)
	require.NoError(t, err)
	require.NoError(t, runner.VerifyDone())
}
