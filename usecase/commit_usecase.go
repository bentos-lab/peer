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
	u.logger.Infof("Commit execution started.")

	if request.Environment == nil {
		return CommitExecutionResult{}, errors.New("code environment is required")
	}

	message := strings.TrimSpace(request.CommitMessage)
	if message == "" {
		generateStartedAt := time.Now()
		generated, err := u.generator.GenerateCommitMessage(ctx, CommitMessagePayload{
			Staged:      !request.StageAll,
			Environment: request.Environment,
		})
		if err != nil {
			u.logger.Errorf(
				"Commit stage %q failed after %d ms: %v.",
				"generate_commit_message",
				time.Since(generateStartedAt).Milliseconds(),
				err,
			)
			return CommitExecutionResult{}, err
		}
		message = strings.TrimSpace(generated)
		if message == "" {
			err := fmt.Errorf("commit message is required")
			u.logger.Errorf(
				"Commit stage %q failed after %d ms: %v.",
				"generate_commit_message",
				time.Since(generateStartedAt).Milliseconds(),
				err,
			)
			return CommitExecutionResult{}, err
		}
		u.logger.Debugf(
			"Commit stage %q completed in %d ms.",
			"generate_commit_message",
			time.Since(generateStartedAt).Milliseconds(),
		)
	}

	if !request.Commit {
		u.logger.Infof(
			"Commit execution completed in %d ms.",
			time.Since(startedAt).Milliseconds(),
		)
		return CommitExecutionResult{CommitMessage: message, Committed: false}, nil
	}

	commitStartedAt := time.Now()
	result, err := request.Environment.CommitChanges(ctx, domain.CodeEnvironmentCommitOptions{
		CommitMessage: message,
		StageAll:      request.StageAll,
	})
	if err != nil {
		u.logger.Errorf(
			"Commit stage %q failed after %d ms: %v.",
			"commit_changes",
			time.Since(commitStartedAt).Milliseconds(),
			err,
		)
		return CommitExecutionResult{}, err
	}
	u.logger.Debugf(
		"Commit stage %q completed in %d ms.",
		"commit_changes",
		time.Since(commitStartedAt).Milliseconds(),
	)

	u.logger.Infof(
		"Commit execution completed in %d ms.",
		time.Since(startedAt).Milliseconds(),
	)
	return CommitExecutionResult{CommitMessage: message, Committed: result.Committed}, nil
}
