package github

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsInvalidAnchorErrorText(t *testing.T) {
	testCases := []struct {
		name string
		text string
		want bool
	}{
		{name: "line in diff", text: "line must be part of the diff", want: true},
		{name: "start line in diff", text: "start_line must be part of the diff", want: true},
		{name: "outside diff", text: "line 20 is outside the diff", want: true},
		{name: "path missing", text: "pull_request_review_thread.path is missing", want: true},
		{name: "validation failed", text: "Validation Failed (HTTP 422)", want: true},
		{name: "unrelated", text: "permission denied", want: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, isInvalidAnchorErrorText(tc.text))
		})
	}
}

func TestIsInvalidAnchorCommandError(t *testing.T) {
	require.True(t, isInvalidAnchorCommandError(errors.New("HTTP 422: line must be part of the diff")))
	require.True(t, isInvalidAnchorCommandError(errors.New("gh: Validation Failed (HTTP 422).")))
	require.False(t, isInvalidAnchorCommandError(errors.New("HTTP 500: line must be part of the diff")))
}

func TestInvalidAnchorAPIErrorClassifier(t *testing.T) {
	require.True(t, isInvalidAnchorAPIError(errors.New("github API request failed with status 422: path is missing")))
	require.True(t, isInvalidAnchorAPIError(errors.New("github API request failed with status 422: Validation Failed")))
	require.False(t, isInvalidAnchorAPIError(errors.New("github API request failed with status 500: path is missing")))
}
