package host

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strings"
)

func (e *HostCodeEnvironment) verifyLocalRefExists(ctx context.Context, workspaceDir string, ref string) error {
	_, err := e.git(ctx, workspaceDir, "rev-parse", "--verify", fmt.Sprintf("%s^{commit}", ref))
	if err != nil {
		return fmt.Errorf("failed to verify ref %q: %w", ref, err)
	}
	return nil
}

func (e *HostCodeEnvironment) normalizeRef(ctx context.Context, workspaceDir string, ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if isWorkspaceTokenRef(ref) {
		return ref, nil
	}
	if err := e.verifyLocalRefExists(ctx, workspaceDir, ref); err == nil {
		return ref, nil
	}
	resolvedRef, err := e.resolveRefFromMissing(ctx, workspaceDir, ref)
	if err != nil {
		return "", err
	}
	if resolvedRef != ref {
		e.logger.Debugf("Normalized ref: requested=%s resolved=%s", ref, resolvedRef)
	}
	return resolvedRef, nil
}

func (e *HostCodeEnvironment) resolveRefFromMissing(ctx context.Context, workspaceDir string, requestedRef string) (string, error) {
	fetchedRef := localFetchedRefName(requestedRef)
	if err := e.verifyLocalRefExists(ctx, workspaceDir, fetchedRef); err == nil {
		e.logger.Debugf("Ref found in fetched cache: requested=%s, resolved=%s", requestedRef, fetchedRef)
		return fetchedRef, nil
	}

	candidates := refFetchCandidates(requestedRef)
	var lastErr error
	isShallow, shallowErr := e.isShallowRepository(ctx, workspaceDir)
	if shallowErr != nil {
		e.logger.Debugf("Failed to determine if repository is shallow: %v", shallowErr)
	}
	for _, candidate := range candidates {
		e.logger.Debugf("Attempting to fetch ref candidate: %s, will store as: %s", candidate, fetchedRef)
		args := []string{"-C", workspaceDir, "fetch"}
		if shallowErr == nil && isShallow {
			args = append(args, "--unshallow")
		}
		args = append(args, "origin", fmt.Sprintf("%s:%s", candidate, fetchedRef))
		result, fetchErr := e.runner.Run(ctx, "git", args...)
		if fetchErr != nil {
			lastErr = formatCommandError(fetchErr, result)
			continue
		}
		if err := e.verifyLocalRefExists(ctx, workspaceDir, fetchedRef); err == nil {
			e.logger.Debugf("Successfully fetched ref: candidate=%s -> %s", candidate, fetchedRef)
			return fetchedRef, nil
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("unknown fetch error")
	}
	return "", fmt.Errorf("failed to resolve ref %q in local workspace: %w", requestedRef, lastErr)
}

func refFetchCandidates(requestedRef string) []string {
	requestedRef = strings.TrimSpace(requestedRef)
	candidates := []string{requestedRef}
	if !strings.HasPrefix(requestedRef, "refs/") {
		candidates = append(candidates, "refs/heads/"+requestedRef, "refs/tags/"+requestedRef)
	}
	return dedupePaths(candidates)
}

func localFetchedRefName(requestedRef string) string {
	sum := sha1.Sum([]byte(strings.TrimSpace(requestedRef)))
	return hostCodeEnvironmentFetchedRefPrefix + hex.EncodeToString(sum[:])
}

func isWorkspaceTokenRef(ref string) bool {
	switch strings.TrimSpace(ref) {
	case "", "@staged", "@all":
		return true
	default:
		return false
	}
}
