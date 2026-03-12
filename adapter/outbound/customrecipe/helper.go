package customrecipe

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"bentos-backend/domain"
	"bentos-backend/usecase"
	uccontracts "bentos-backend/usecase/contracts"
)

func (l *Loader) readAndSanitize(ctx context.Context, env uccontracts.CodeEnvironment, headRef string, rawPath string, sanitizer usecase.SafetySanitizer) (string, string, error) {
	path, err := resolveRecipePath(rawPath)
	if err != nil {
		l.logger.Warnf("Custom recipe path %q is invalid: %v", rawPath, err)
		return "", "", nil
	}
	if path == "" {
		return "", "", nil
	}

	fullPath := filepath.Join(".autogit", path)
	content, found, err := env.ReadFile(ctx, fullPath, headRef)
	if err != nil {
		return "", "", err
	}
	if !found {
		l.logger.Warnf("Custom recipe file %q was not found.", fullPath)
		return "", fullPath, nil
	}

	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return "", "", nil
	}
	result, err := sanitizer.Sanitize(ctx, trimmed)
	if err != nil {
		return "", "", err
	}
	if result.Status != domain.PromptSafetyStatusOK {
		l.logger.Warnf("Custom recipe file %q was rejected by sanitizer.", fullPath)
		return "", "", nil
	}
	return strings.TrimSpace(result.SanitizedPrompt), "", nil
}

func resolveRecipePath(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", nil
	}
	if filepath.IsAbs(trimmed) {
		return "", fmt.Errorf("path must be relative to .autogit")
	}
	cleaned := filepath.Clean(trimmed)
	if cleaned == "." {
		return "", fmt.Errorf("path must be a file")
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path must remain within .autogit")
	}
	return cleaned, nil
}
