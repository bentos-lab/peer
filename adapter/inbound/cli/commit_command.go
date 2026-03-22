package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	codeenv "github.com/bentos-lab/peer/adapter/outbound/codeenv"
	"github.com/bentos-lab/peer/config"
	"github.com/bentos-lab/peer/shared/logger/stdlogger"
	sharedlogging "github.com/bentos-lab/peer/shared/logging"
	"github.com/bentos-lab/peer/usecase"
	uccontracts "github.com/bentos-lab/peer/usecase/contracts"
)

// CommitCommand runs commit flow with the shared commit usecase.
type CommitCommand struct {
	commitUseCaseBuilder CommitUseCaseBuilder
	envFactory           uccontracts.CodeEnvironmentFactory
	logger               usecase.Logger
	promptYesNo          func(io.Writer, io.Reader, string) (bool, error)
}

// CommitUseCaseBuilder builds a commit usecase for a specific repo.
type CommitUseCaseBuilder func(repoURL string) (usecase.CommitUseCase, error)

// CommitRunParams contains already-parsed CLI commit parameters.
type CommitRunParams struct {
	Staged  bool
	Confirm *bool
}

// NewCommitCommand creates a new CLI command for commit.
func NewCommitCommand(commitUseCaseBuilder CommitUseCaseBuilder, envFactory uccontracts.CodeEnvironmentFactory, logger usecase.Logger) *CommitCommand {
	if logger == nil {
		logger = stdlogger.Nop()
	}
	return &CommitCommand{
		commitUseCaseBuilder: commitUseCaseBuilder,
		envFactory:           envFactory,
		logger:               logger,
		promptYesNo:          promptYesNo,
	}
}

// Run executes the CLI commit flow.
func (c *CommitCommand) Run(ctx context.Context, cfg config.Config, params CommitRunParams, stdout io.Writer, stdin io.Reader) error {
	if c.commitUseCaseBuilder == nil {
		return errors.New("commit usecase is not configured")
	}
	_ = cfg
	if c.envFactory == nil {
		return errors.New("code environment factory is not configured")
	}
	if c.logger == nil {
		c.logger = stdlogger.Nop()
	}
	if stdout == nil {
		return errors.New("stdout is required")
	}
	if stdin == nil {
		return errors.New("stdin is required")
	}

	environment, cleanup, err := codeenv.NewEnvironment(ctx, c.envFactory, "")
	if err != nil {
		return err
	}
	defer func() {
		if cleanupErr := cleanup(ctx); cleanupErr != nil {
			c.logger.Warnf("Failed to cleanup code environment: %v", cleanupErr)
		}
	}()

	commitUseCase, err := c.commitUseCaseBuilder("")
	if err != nil {
		return err
	}

	request := usecase.CommitRequest{
		Commit:      false,
		StageAll:    !params.Staged,
		Environment: environment,
	}

	startedAt := time.Now()
	c.logger.Infof("CLI commit started.")
	sharedlogging.LogInputSnapshot(c.logger, "cli", "", request)

	result, err := commitUseCase.Execute(ctx, request)
	if err != nil {
		c.logger.Errorf("CLI commit message generation failed.")
		c.logger.Debugf("The CLI commit ran for %d ms before failing.", time.Since(startedAt).Milliseconds())
		c.logger.Debugf("Failure details: %v.", err)
		return err
	}

	_, _ = fmt.Fprintln(stdout, result.CommitMessage)

	if params.Confirm == nil {
		ok, err := c.promptYesNo(stdout, stdin, "Commit with this message? [y/N]: ")
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
	} else if !*params.Confirm {
		return nil
	}

	request.Commit = true
	request.CommitMessage = result.CommitMessage
	commitResult, err := commitUseCase.Execute(ctx, request)
	if err != nil {
		c.logger.Errorf("CLI commit failed.")
		c.logger.Debugf("The CLI commit ran for %d ms before failing.", time.Since(startedAt).Milliseconds())
		c.logger.Debugf("Failure details: %v.", err)
		return err
	}

	c.logger.Infof("CLI commit completed.")
	c.logger.Debugf("The CLI commit completed in %d ms.", time.Since(startedAt).Milliseconds())
	_ = commitResult
	return nil
}

func promptYesNo(writer io.Writer, reader io.Reader, prompt string) (bool, error) {
	if writer == nil || reader == nil {
		return false, errors.New("prompt requires io")
	}
	_, _ = fmt.Fprint(writer, prompt)
	buf := bufio.NewReader(reader)
	line, err := buf.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	line = strings.TrimSpace(strings.ToLower(line))
	switch line {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}
