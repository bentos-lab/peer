package codingagent

import (
	"context"
	"fmt"
	"strings"

	"bentos-backend/domain"
	uccontracts "bentos-backend/usecase/contracts"
)

func normalizePromptRefs(base string, head string) (string, string) {
	normalizedBase := strings.TrimSpace(base)
	normalizedHead := strings.TrimSpace(head)
	if normalizedHead == "@staged" || normalizedHead == "@all" {
		return "", normalizedHead
	}
	return normalizedBase, normalizedHead
}

func ensureDiffContentAvailable(ctx context.Context, environment uccontracts.CodeEnvironment, base string, head string) error {
	changedFiles, err := environment.LoadChangedFiles(ctx, domain.CodeEnvironmentLoadOptions{
		Base: base,
		Head: head,
	})
	if err != nil {
		return fmt.Errorf("failed to load changed files: %w", err)
	}
	for _, file := range changedFiles {
		if strings.TrimSpace(file.DiffSnippet) != "" {
			return nil
		}
	}
	return fmt.Errorf("diff content is empty for base %q and head %q", base, head)
}

func runTask(ctx context.Context, agent uccontracts.CodingAgent, cfg Config, task string) (string, error) {
	result, err := agent.Run(ctx, strings.TrimSpace(task), domain.CodingAgentRunOptions{
		Provider: cfg.Provider,
		Model:    cfg.Model,
	})
	if err != nil {
		return "", fmt.Errorf("failed to run coding agent task: %w", err)
	}
	return strings.TrimSpace(result.Text), nil
}
