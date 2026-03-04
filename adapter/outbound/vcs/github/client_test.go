package github

import (
	"context"
	"errors"
	"testing"

	"bentos-backend/adapter/outbound/commandrunner"
	"github.com/stretchr/testify/require"
)

func newTestClient(runner commandrunner.Runner) *Client {
	return &Client{runner: runner}
}

func TestClient_GetPullRequestChangedFiles(t *testing.T) {
	runner := commandrunner.NewDummyCommandRunner()
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{Name: "gh", Args: []string{"auth", "status"}},
		Result:   commandrunner.Result{Stdout: []byte("ok")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{Name: "gh", Args: []string{"api", "repos/org/repo/pulls/7/files", "--paginate"}},
		Result: commandrunner.Result{Stdout: []byte(`[
			{"filename":"a.go","patch":"@@ -1 +1 @@\n-old\n+new"},
			{"filename":"b.png"}
		]`)},
	})
	client := newTestClient(runner)

	files, err := client.GetPullRequestChangedFiles(context.Background(), "org/repo", 7)
	require.NoError(t, err)
	require.Len(t, files, 1)
	require.Equal(t, "a.go", files[0].Path)
	require.Contains(t, files[0].Content, "+new")
	require.NoError(t, runner.VerifyDone())
}

func TestClient_GetPullRequestChangedFilesResolvesRepositoryWhenMissing(t *testing.T) {
	runner := commandrunner.NewDummyCommandRunner()
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{Name: "gh", Args: []string{"auth", "status"}},
		Result:   commandrunner.Result{Stdout: []byte("ok")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{Name: "gh", Args: []string{"repo", "view", "--json", "nameWithOwner"}},
		Result:   commandrunner.Result{Stdout: []byte(`{"nameWithOwner":"org/repo"}`)},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{Name: "gh", Args: []string{"api", "repos/org/repo/pulls/12/files", "--paginate"}},
		Result:   commandrunner.Result{Stdout: []byte(`[]`)},
	})
	client := newTestClient(runner)

	_, err := client.GetPullRequestChangedFiles(context.Background(), "", 12)
	require.NoError(t, err)
	require.Len(t, runner.Calls(), 3)
	require.NoError(t, runner.VerifyDone())
}

func TestClient_GetPullRequestChangedFilesFailsWhenGitHubCLIUnauthorized(t *testing.T) {
	runner := commandrunner.NewDummyCommandRunner()
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{Name: "gh", Args: []string{"auth", "status"}},
		Result:   commandrunner.Result{Stderr: []byte("not logged in")},
		Err:      errors.New("exit status 1"),
	})
	client := newTestClient(runner)

	_, err := client.GetPullRequestChangedFiles(context.Background(), "org/repo", 7)
	require.Error(t, err)
	require.Contains(t, err.Error(), "gh auth login")
	require.NoError(t, runner.VerifyDone())
}

func TestClient_CreateComment(t *testing.T) {
	runner := commandrunner.NewDummyCommandRunner()
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{Name: "gh", Args: []string{"auth", "status"}},
		Result:   commandrunner.Result{Stdout: []byte("ok")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{Name: "gh", Args: []string{"pr", "comment", "21", "--repo", "org/repo", "--body", "hello"}},
		Result:   commandrunner.Result{Stdout: []byte("ok")},
	})
	client := newTestClient(runner)

	err := client.CreateComment(context.Background(), "org/repo", 21, "hello")
	require.NoError(t, err)
	require.Len(t, runner.Calls(), 2)
	require.NoError(t, runner.VerifyDone())
}

func TestClient_CreateCommentResolvesRepositoryWhenMissing(t *testing.T) {
	runner := commandrunner.NewDummyCommandRunner()
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{Name: "gh", Args: []string{"auth", "status"}},
		Result:   commandrunner.Result{Stdout: []byte("ok")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{Name: "gh", Args: []string{"repo", "view", "--json", "nameWithOwner"}},
		Result:   commandrunner.Result{Stdout: []byte(`{"nameWithOwner":"org/repo"}`)},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{Name: "gh", Args: []string{"pr", "comment", "3", "--repo", "org/repo", "--body", "body"}},
		Result:   commandrunner.Result{Stdout: []byte("ok")},
	})
	client := newTestClient(runner)

	err := client.CreateComment(context.Background(), "", 3, "body")
	require.NoError(t, err)
	require.Len(t, runner.Calls(), 3)
	require.NoError(t, runner.VerifyDone())
}

func TestClient_GetPullRequestChangedFilesParsesPaginatedMultiDocumentOutput(t *testing.T) {
	runner := commandrunner.NewDummyCommandRunner()
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{Name: "gh", Args: []string{"auth", "status"}},
		Result:   commandrunner.Result{Stdout: []byte("ok")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{Name: "gh", Args: []string{"api", "repos/org/repo/pulls/7/files", "--paginate"}},
		Result: commandrunner.Result{Stdout: []byte(`
[{"filename":"a.go","patch":"@@ -1 +1 @@\n-old\n+new"}]
[{"filename":"b.go","patch":"@@ -2 +2 @@\n-old2\n+new2"}]
`)},
	})
	client := newTestClient(runner)

	files, err := client.GetPullRequestChangedFiles(context.Background(), "org/repo", 7)
	require.NoError(t, err)
	require.Len(t, files, 2)
	require.Equal(t, "a.go", files[0].Path)
	require.Equal(t, "b.go", files[1].Path)
	require.NoError(t, runner.VerifyDone())
}

func TestClient_CreateReviewCommentSingleLine(t *testing.T) {
	runner := commandrunner.NewDummyCommandRunner()
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{Name: "gh", Args: []string{"auth", "status"}},
		Result:   commandrunner.Result{Stdout: []byte("ok")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{Name: "gh", Args: []string{"api", "repos/org/repo/pulls/21"}},
		Result:   commandrunner.Result{Stdout: []byte(`{"head":{"sha":"abc123"}}`)},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{Name: "gh", Args: []string{
			"api", "--method", "POST", "repos/org/repo/pulls/21/comments",
			"--raw-field", "body=hello",
			"--raw-field", "path=service.go",
			"--raw-field", "side=RIGHT",
			"--raw-field", "commit_id=abc123",
			"--field", "line=9",
		}},
		Result: commandrunner.Result{Stdout: []byte("ok")},
	})
	client := newTestClient(runner)

	err := client.CreateReviewComment(context.Background(), "org/repo", 21, CreateReviewCommentInput{
		Body:      "hello",
		Path:      "service.go",
		StartLine: 9,
		EndLine:   9,
	})
	require.NoError(t, err)
	require.NoError(t, runner.VerifyDone())
}

func TestClient_CreateReviewCommentRangeAndResolveRepo(t *testing.T) {
	runner := commandrunner.NewDummyCommandRunner()
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{Name: "gh", Args: []string{"auth", "status"}},
		Result:   commandrunner.Result{Stdout: []byte("ok")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{Name: "gh", Args: []string{"repo", "view", "--json", "nameWithOwner"}},
		Result:   commandrunner.Result{Stdout: []byte(`{"nameWithOwner":"org/repo"}`)},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{Name: "gh", Args: []string{"api", "repos/org/repo/pulls/3"}},
		Result:   commandrunner.Result{Stdout: []byte(`{"head":{"sha":"def456"}}`)},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{Name: "gh", Args: []string{
			"api", "--method", "POST", "repos/org/repo/pulls/3/comments",
			"--raw-field", "body=body",
			"--raw-field", "path=a.go",
			"--raw-field", "side=RIGHT",
			"--raw-field", "commit_id=def456",
			"--field", "line=12",
			"--field", "start_line=10",
			"--raw-field", "start_side=RIGHT",
		}},
		Result: commandrunner.Result{Stdout: []byte("ok")},
	})
	client := newTestClient(runner)

	err := client.CreateReviewComment(context.Background(), "", 3, CreateReviewCommentInput{
		Body:      "body",
		Path:      "a.go",
		StartLine: 10,
		EndLine:   12,
	})
	require.NoError(t, err)
	require.NoError(t, runner.VerifyDone())
}

func TestClient_CreateReviewCommentClassifiesInvalidAnchor(t *testing.T) {
	runner := commandrunner.NewDummyCommandRunner()
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{Name: "gh", Args: []string{"auth", "status"}},
		Result:   commandrunner.Result{Stdout: []byte("ok")},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{Name: "gh", Args: []string{"api", "repos/org/repo/pulls/7"}},
		Result:   commandrunner.Result{Stdout: []byte(`{"head":{"sha":"abc123"}}`)},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{Name: "gh", Args: []string{
			"api", "--method", "POST", "repos/org/repo/pulls/7/comments",
			"--raw-field", "body=bad",
			"--raw-field", "path=a.go",
			"--raw-field", "side=RIGHT",
			"--raw-field", "commit_id=abc123",
			"--field", "line=90",
		}},
		Result: commandrunner.Result{Stderr: []byte("HTTP 422: line must be part of the diff")},
		Err:    errors.New("exit status 1"),
	})
	client := newTestClient(runner)

	err := client.CreateReviewComment(context.Background(), "org/repo", 7, CreateReviewCommentInput{
		Body:      "bad",
		Path:      "a.go",
		StartLine: 90,
		EndLine:   90,
	})
	require.Error(t, err)
	require.True(t, IsInvalidAnchorError(err))
	require.NoError(t, runner.VerifyDone())
}
