package host

import (
	"context"
	"fmt"
	"strings"

	"github.com/bentos-lab/peer/domain"
)

// EnsureDiffContentAvailable validates that diff content is present for the requested comparison.
func (e *HostCodeEnvironment) EnsureDiffContentAvailable(ctx context.Context, opts domain.CodeEnvironmentLoadOptions) error {
	changedFiles, err := e.LoadChangedFiles(ctx, opts)
	if err != nil {
		return fmt.Errorf("failed to load changed files: %w", err)
	}
	for _, file := range changedFiles {
		if strings.TrimSpace(file.DiffSnippet) != "" {
			return nil
		}
	}
	return fmt.Errorf("diff content is empty for base %q and head %q", opts.Base, opts.Head)
}
