package codingagent

import (
	"fmt"
	"strings"
	"time"

	"github.com/bentos-lab/peer/domain"
)

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
