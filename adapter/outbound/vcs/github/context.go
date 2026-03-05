package github

import (
	"context"
	"strings"
)

type installationIDContextKey struct{}

// WithInstallationID stores the GitHub App installation ID in context.
func WithInstallationID(ctx context.Context, installationID string) context.Context {
	return context.WithValue(ctx, installationIDContextKey{}, strings.TrimSpace(installationID))
}

// InstallationIDFromContext returns the GitHub App installation ID from context.
func InstallationIDFromContext(ctx context.Context) string {
	return installationIDFromContext(ctx)
}

func installationIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	value, _ := ctx.Value(installationIDContextKey{}).(string)
	return strings.TrimSpace(value)
}
