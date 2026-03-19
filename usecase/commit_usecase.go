package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bentos-lab/peer/domain"
	"github.com/bentos-lab/peer/shared/logger/stdlogger"
)

// commitUseCase is the concrete CommitUseCase implementation.
type commitUseCase struct {
	generator CommitMessageGenerator
	logger    Logger
}

// NewCommitUseCase constructs a commit usecase.
func NewCommitUseCase(generator CommitMessageGenerator, logger Logger) (CommitUseCase, error) {
	if generator == nil {
		return nil, errors.New("commit usecase dependencies must not be nil")
	}
	if logger == nil {
		logger = stdlogger.Nop()
	}
	return &commitUseCase{generator: generator, logger: logger}, nil
}

// Execute runs the commit flow.
func (u *commitUseCase) Execute(ctx context.Context, request CommitRequest) (CommitExecutionResult, error) {
	startedAt := time.Now()
	target := request.Input.Target
	logExecution(u.logger, "commit", target, "start", startedAt, "")

	if request.Environment == nil {
		return CommitExecutionResult{}, errors.New("code environment is required")
	}

	message := strings.TrimSpace(request.CommitMessage)
	if message == "" {
		generateStartedAt := time.Now()
		generated, err := u.generator.GenerateCommitMessage(ctx, CommitMessagePayload{
			Input:       request.Input,
			Staged:      !request.StageAll,
			Environment: request.Environment,
		})
		if err != nil {
			logStage(u.logger, "commit", "generate_commit_message", target, "failure", generateStartedAt, "%v", err)
			return CommitExecutionResult{}, err
		}
		message = strings.TrimSpace(generated)
		if message == "" {
			err := fmt.Errorf("commit message is required")
			logStage(u.logger, "commit", "generate_commit_message", target, "failure", generateStartedAt, "%v", err)
			return CommitExecutionResult{}, err
		}
		logStage(u.logger, "commit", "generate_commit_message", target, "success", generateStartedAt, "")
	}

	if !request.Commit {
		logExecution(u.logger, "commit", target, "complete", startedAt, "")
		return CommitExecutionResult{CommitMessage: message, Committed: false}, nil
	}

	commitStartedAt := time.Now()
	result, err := request.Environment.CommitChanges(ctx, domain.CodeEnvironmentCommitOptions{
		CommitMessage: message,
		StageAll:      request.StageAll,
	})
	if err != nil {
		logStage(u.logger, "commit", "commit_changes", target, "failure", commitStartedAt, "%v", err)
		return CommitExecutionResult{}, err
	}
	logStage(u.logger, "commit", "commit_changes", target, "success", commitStartedAt, "")

	logExecution(u.logger, "commit", target, "complete", startedAt, "")
	return CommitExecutionResult{CommitMessage: message, Committed: result.Committed}, nil
}
