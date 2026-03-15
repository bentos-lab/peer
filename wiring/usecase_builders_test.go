package wiring

import (
	"testing"

	"bentos-backend/config"
	"github.com/stretchr/testify/require"
)

func TestBuildChangeRequestUseCaseRejectsMissingCodingAgent(t *testing.T) {
	_, err := BuildChangeRequestUseCase(config.Config{}, CLILLMOptions{ForceCLIPublishers: true}, "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "coding agent")
}

func TestBuildAutogenUseCaseRejectsMissingCodingAgent(t *testing.T) {
	_, err := BuildAutogenUseCase(config.Config{}, CLILLMOptions{ForceCLIPublishers: true}, "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "coding agent")
}

func TestBuildReplyCommentUseCaseRejectsMissingCodingAgent(t *testing.T) {
	_, err := BuildReplyCommentUseCase(config.Config{}, CLILLMOptions{ForceCLIPublishers: true}, "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "coding agent")
}
