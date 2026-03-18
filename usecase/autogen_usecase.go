package usecase

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/bentos-lab/peer/domain"
	diffutil "github.com/bentos-lab/peer/shared/diff"
	"github.com/bentos-lab/peer/shared/logger/stdlogger"
)

// autogenUseCase is the concrete AutogenUseCase implementation.
type autogenUseCase struct {
	generator AutogenGenerator
	publisher AutogenPublisher
	logger    Logger
}

// NewAutogenUseCase constructs an autogen usecase.
func NewAutogenUseCase(
	generator AutogenGenerator,
	publisher AutogenPublisher,
	logger Logger,
) (AutogenUseCase, error) {
	if generator == nil || publisher == nil {
		return nil, errors.New("autogen usecase dependencies must not be nil")
	}
	if logger == nil {
		logger = stdlogger.Nop()
	}
	return &autogenUseCase{
		generator: generator,
		publisher: publisher,
		logger:    logger,
	}, nil
}

// Execute runs the autogen flow.
func (u *autogenUseCase) Execute(ctx context.Context, request AutogenRequest) (AutogenExecutionResult, error) {
	startedAt := time.Now()
	target := request.Input.Target
	logExecution(u.logger, "autogen", target, "start", startedAt, "")

	if request.Environment == nil {
		return AutogenExecutionResult{}, errors.New("code environment is required")
	}

	if request.Publish {
		if target.ChangeRequestNumber <= 0 {
			return AutogenExecutionResult{}, fmt.Errorf("autogen publish requires change request number")
		}
		if strings.TrimSpace(request.HeadBranch) == "" {
			return AutogenExecutionResult{}, fmt.Errorf("autogen publish requires head branch")
		}
	}
	if !request.Docs && !request.Tests {
		return AutogenExecutionResult{}, fmt.Errorf("autogen requires --docs and/or --tests")
	}

	generateStartedAt := time.Now()
	agentOutput, err := u.generator.Generate(ctx, AutogenPayload{
		Input:         request.Input,
		Docs:          request.Docs,
		Tests:         request.Tests,
		HeadBranch:    request.HeadBranch,
		Environment:   request.Environment,
		ExtraGuidance: strings.TrimSpace(request.Recipe.AutogenGuidance),
	})
	if err != nil {
		logStage(u.logger, "autogen", "generate_autogen", target, "failure", generateStartedAt, "%v", err)
		return AutogenExecutionResult{}, err
	}
	if request.Publish && strings.TrimSpace(agentOutput) == "" {
		return AutogenExecutionResult{}, fmt.Errorf("autogen publish requires agent output")
	}
	logStage(u.logger, "autogen", "generate_autogen", target, "success", generateStartedAt, "")

	collectStartedAt := time.Now()
	changedFiles, err := request.Environment.LoadChangedFiles(ctx, domain.CodeEnvironmentLoadOptions{
		Head: "@all",
	})
	if err != nil {
		if errors.Is(err, domain.ErrNoCodeChanges) {
			u.logger.Infof("No autogen changes detected; skipping content summary.")
			changedFiles = nil
		} else {
			logStage(u.logger, "autogen", "collect_autogen_changes", target, "failure", collectStartedAt, "%v", err)
			return AutogenExecutionResult{}, err
		}
	}
	changes := buildAutogenChanges(changedFiles)
	summary := buildAutogenSummary(changes)
	logStage(u.logger, "autogen", "collect_autogen_changes", target, "success", collectStartedAt, "")

	publishStartedAt := time.Now()
	if err := u.publisher.PublishAutogen(ctx, AutogenPublishRequest{
		Target:      target,
		Changes:     changes,
		Summary:     summary,
		Publish:     request.Publish,
		HeadBranch:  request.HeadBranch,
		Metadata:    request.Input.Metadata,
		AgentOutput: agentOutput,
		Environment: request.Environment,
		PushOptions: domain.CodeEnvironmentPushOptions{
			TargetBranch:  request.HeadBranch,
			CommitMessage: "autogen: add tests/docs/comments",
			RemoteName:    "origin",
		},
		RecipeWarnings: request.Recipe.MissingPaths,
	}); err != nil {
		logStage(u.logger, "autogen", "publish_autogen_result", target, "failure", publishStartedAt, "%v", err)
		return AutogenExecutionResult{}, err
	}
	logStage(u.logger, "autogen", "publish_autogen_result", target, "success", publishStartedAt, "")

	logExecution(
		u.logger,
		"autogen",
		target,
		"complete",
		startedAt,
		"Autogen execution took %d ms and produced %d change blocks.",
		time.Since(startedAt).Milliseconds(),
		len(changes),
	)

	return AutogenExecutionResult{Changes: changes, Summary: summary, AgentOutput: agentOutput}, nil
}

func buildAutogenChanges(files []domain.ChangedFile) []domain.AutogenChange {
	changes := make([]domain.AutogenChange, 0)
	for _, file := range files {
		blocks := diffutil.ExtractAddedBlocks(file.DiffSnippet)
		if len(blocks) == 0 {
			content := strings.TrimSpace(file.Content)
			if content == "" {
				continue
			}
			lines := strings.Split(content, "\n")
			end := len(lines)
			if end == 0 {
				continue
			}
			changes = append(changes, domain.AutogenChange{
				FilePath:  strings.TrimSpace(file.Path),
				StartLine: 1,
				EndLine:   end,
				Content:   content,
			})
			continue
		}
		for _, block := range blocks {
			if block.StartLine <= 0 || block.EndLine < block.StartLine {
				continue
			}
			changes = append(changes, domain.AutogenChange{
				FilePath:  strings.TrimSpace(file.Path),
				StartLine: block.StartLine,
				EndLine:   block.EndLine,
				Content:   block.Content,
			})
		}
	}
	return changes
}

func buildAutogenSummary(changes []domain.AutogenChange) domain.AutogenSummary {
	testFiles := map[string]struct{}{}
	docFiles := map[string]struct{}{}
	commentFiles := map[string]struct{}{}

	for _, change := range changes {
		path := strings.TrimSpace(change.FilePath)
		if path == "" {
			continue
		}
		isDoc := isDocPath(path)
		if strings.HasSuffix(path, "_test.go") {
			testFiles[path] = struct{}{}
		}
		if isDoc {
			docFiles[path] = struct{}{}
		}
		if !isDoc && hasCommentLine(change.Content) {
			commentFiles[path] = struct{}{}
		}
	}

	return domain.AutogenSummary{
		Tests:    sortedKeys(testFiles),
		Docs:     sortedKeys(docFiles),
		Comments: sortedKeys(commentFiles),
	}
}

func sortedKeys(source map[string]struct{}) []string {
	result := make([]string, 0, len(source))
	for key := range source {
		result = append(result, key)
	}
	sort.Strings(result)
	return result
}

func isDocPath(path string) bool {
	if strings.HasPrefix(path, "docs/") {
		return true
	}
	return strings.HasSuffix(path, ".md")
}

func hasCommentLine(content string) bool {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") {
			return true
		}
		if strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "*/") {
			return true
		}
		if strings.HasPrefix(trimmed, "*") {
			return true
		}
	}
	return false
}
