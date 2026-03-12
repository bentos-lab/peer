package codingagent

import (
	"context"
	"fmt"
	"strings"
	"time"

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

func formatThread(thread domain.CommentThread) string {
	if len(thread.Context) == 0 && len(thread.Comments) == 0 {
		return "(no prior comments)"
	}
	var builder strings.Builder
	if len(thread.Context) > 0 {
		builder.WriteString("Parent context:\n")
		for _, line := range thread.Context {
			line = strings.TrimRight(line, " ")
			if line == "" {
				builder.WriteString("\n")
				continue
			}
			builder.WriteString(line)
			builder.WriteString("\n")
		}
		builder.WriteString("\n")
	}
	for _, comment := range thread.Comments {
		author := strings.TrimSpace(comment.Author.Login)
		if author == "" {
			author = "unknown"
		}
		builder.WriteString(fmt.Sprintf("- [%s] %s\n", comment.CreatedAt.Format(time.RFC3339), author))
		body := strings.TrimSpace(comment.Body)
		if body == "" {
			body = "(empty)"
		}
		for _, line := range strings.Split(body, "\n") {
			builder.WriteString(fmt.Sprintf("  %s\n", line))
		}
	}
	return strings.TrimSpace(builder.String())
}
