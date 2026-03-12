package host

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"bentos-backend/adapter/outbound/commandrunner"
	"bentos-backend/shared/logger/stdlogger"
	"bentos-backend/shared/toolinstall"
	"bentos-backend/usecase"
)

func resolveHostDefaults(
	runner commandrunner.Runner,
	agentRunner commandrunner.StreamRunner,
	getwd func() (string, error),
	makeTempDir func() (string, error),
	logger usecase.Logger,
) hostDefaults {
	if runner == nil {
		runner = commandrunner.NewOSCommandRunner()
	}
	if agentRunner == nil {
		agentRunner = commandrunner.NewOSStreamCommandRunner()
	}
	if getwd == nil {
		getwd = os.Getwd
	}
	if makeTempDir == nil {
		makeTempDir = newHostCodeEnvironmentTempDirMaker(os.UserHomeDir)
	}
	if logger == nil {
		logger = stdlogger.Nop()
	}
	return hostDefaults{
		runner:      runner,
		agentRunner: agentRunner,
		getwd:       getwd,
		makeTempDir: makeTempDir,
		logger:      logger,
	}
}

func (e *HostCodeEnvironment) verifyLocalRefExists(ctx context.Context, workspaceDir string, ref string) error {
	_, err := e.git(ctx, workspaceDir, "rev-parse", "--verify", fmt.Sprintf("%s^{commit}", ref))
	if err != nil {
		return fmt.Errorf("failed to verify ref %q: %w", ref, err)
	}
	return nil
}

func (e *HostCodeEnvironment) prepareWorkspace(ctx context.Context, repoURL string) error {
	repoURL = strings.TrimSpace(repoURL)
	if repoURL == "" {
		workspaceDir, err := e.getwd()
		if err != nil {
			return fmt.Errorf("failed to resolve current workspace directory: %w", err)
		}
		e.workspaceDir = workspaceDir
		e.isRemote = false
		return nil
	}

	workspaceDir, err := e.makeTempDir()
	if err != nil {
		return fmt.Errorf("failed to create temporary workspace directory: %w", err)
	}
	e.logger.Debugf("Code environment temporary workspace directory is %q under tmp folder %q.", workspaceDir, filepath.Dir(workspaceDir))

	result, err := e.runner.Run(ctx, "git", "clone", "--depth", "1", repoURL, workspaceDir)
	if err != nil {
		return fmt.Errorf("failed to clone repository: %w", formatCommandError(err, result))
	}

	e.logger.Debugf("Cloned repo to %s (shallow=true)", workspaceDir)
	e.workspaceDir = workspaceDir
	e.isRemote = true
	return nil
}

func (e *HostCodeEnvironment) workspaceDirForRun() (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if strings.TrimSpace(e.workspaceDir) == "" {
		workspaceDir, err := e.getwd()
		if err != nil {
			return "", fmt.Errorf("failed to resolve current workspace directory: %w", err)
		}
		e.workspaceDir = workspaceDir
		e.isRemote = false
	}
	return e.workspaceDir, nil
}

func (e *HostCodeEnvironment) syncRef(ctx context.Context, workspaceDir string, headRef string) error {
	if isWorkspaceTokenRef(headRef) {
		return nil
	}

	currentHead, headErr := e.getCurrentHead(ctx, workspaceDir)
	if headErr != nil {
		e.logger.Debugf("Failed to get current HEAD before sync: %v", headErr)
	} else {
		e.logger.Debugf("Current HEAD before sync: %s", currentHead)
	}

	resolvedHeadRef, err := e.resolveRef(ctx, workspaceDir, headRef)
	if err != nil {
		return err
	}

	e.logger.Debugf("Syncing ref: requested=%s, resolved=%s", headRef, resolvedHeadRef)
	_, err = e.git(ctx, workspaceDir, "checkout", resolvedHeadRef)
	if err != nil {
		return fmt.Errorf("failed to checkout ref %q: %w", resolvedHeadRef, err)
	}

	currentHead, headErr = e.getCurrentHead(ctx, workspaceDir)
	if headErr != nil {
		e.logger.Debugf("Failed to get current HEAD after sync: %v", headErr)
	} else {
		e.logger.Debugf("Current HEAD after sync: %s", currentHead)
	}

	return nil
}

func (e *HostCodeEnvironment) resolveRef(ctx context.Context, workspaceDir string, requestedRef string) (string, error) {
	requestedRef = strings.TrimSpace(requestedRef)
	if requestedRef == "" {
		return "", fmt.Errorf("ref is required")
	}
	e.logger.Debugf("Resolving ref: %s (isRemote=%v)", requestedRef, e.isRemote)
	if err := e.verifyLocalRefExists(ctx, workspaceDir, requestedRef); err == nil {
		e.logger.Debugf("Ref found locally: requested=%s", requestedRef)
		return requestedRef, nil
	}

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

func (e *HostCodeEnvironment) listChangedPaths(ctx context.Context, workspaceDir string, args ...string) ([]string, error) {
	result, err := e.git(ctx, workspaceDir, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list changed paths: %w", err)
	}
	lines := strings.Split(strings.TrimSpace(string(result.Stdout)), "\n")
	paths := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		paths = append(paths, line)
	}
	return paths, nil
}

func (e *HostCodeEnvironment) readPathContent(ctx context.Context, workspaceDir string, path string, head string) (string, error) {
	if isWorkspaceTokenRef(head) {
		raw, err := os.ReadFile(filepath.Join(workspaceDir, path))
		if err == nil {
			return string(raw), nil
		}
		e.logger.Debugf("Failed to read file %s from workspace: %v", path, err)

		// For staged content that is missing from the working tree, read from index.
		result, showErr := e.git(ctx, workspaceDir, "show", fmt.Sprintf(":%s", path))
		if showErr != nil {
			return "", fmt.Errorf("failed to read file content for %q: %w (index fallback failed: %v)", path, err, showErr)
		}
		return strings.TrimSpace(string(result.Stdout)), nil
	}
	result, err := e.git(ctx, workspaceDir, "show", fmt.Sprintf("%s:%s", head, path))
	if err != nil {
		return "", fmt.Errorf("failed to read file content for %q at ref %q: %w", path, head, err)
	}
	return strings.TrimSpace(string(result.Stdout)), nil
}

func (e *HostCodeEnvironment) readWorkspaceFile(ctx context.Context, workspaceDir string, path string) (string, bool, error) {
	raw, err := os.ReadFile(filepath.Join(workspaceDir, path))
	if err == nil {
		return string(raw), true, nil
	}
	readErr := err
	if !errors.Is(readErr, os.ErrNotExist) {
		e.logger.Debugf("Failed to read file %s from workspace: %v", path, readErr)
	}

	result, showErr := e.git(ctx, workspaceDir, "show", fmt.Sprintf(":%s", path))
	if showErr != nil {
		if isGitPathMissing(showErr) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("failed to read file content for %q: %w (index fallback failed: %v)", path, readErr, showErr)
	}
	return strings.TrimSpace(string(result.Stdout)), true, nil
}

func (e *HostCodeEnvironment) readRefFile(ctx context.Context, workspaceDir string, path string, ref string) (string, bool, error) {
	_, err := e.git(ctx, workspaceDir, "cat-file", "-e", fmt.Sprintf("%s:%s", ref, path))
	if err != nil {
		if isGitPathMissing(err) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("failed to verify file %q at ref %q: %w", path, ref, err)
	}

	result, err := e.git(ctx, workspaceDir, "show", fmt.Sprintf("%s:%s", ref, path))
	if err != nil {
		if isGitPathMissing(err) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("failed to read file content for %q at ref %q: %w", path, ref, err)
	}
	return strings.TrimSpace(string(result.Stdout)), true, nil
}

func (e *HostCodeEnvironment) getCurrentHead(ctx context.Context, workspaceDir string) (string, error) {
	result, err := e.git(ctx, workspaceDir, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(result.Stdout)), nil
}

func (e *HostCodeEnvironment) mergeBase(ctx context.Context, workspaceDir string, base string, head string) (string, error) {
	result, err := e.git(ctx, workspaceDir, "merge-base", base, head)
	if err != nil {
		return "", fmt.Errorf("failed to resolve merge-base for %q and %q: %w", base, head, err)
	}
	mergeBase := strings.TrimSpace(string(result.Stdout))
	if mergeBase == "" {
		return "", fmt.Errorf("failed to resolve merge-base for %q and %q: empty result", base, head)
	}
	return mergeBase, nil
}

func (e *HostCodeEnvironment) diffForPath(ctx context.Context, workspaceDir string, path string, base string, head string) (string, error) {
	var args []string
	switch strings.TrimSpace(head) {
	case "", "@staged":
		args = []string{"diff", "--cached", "--", path}
	case "@all":
		stagedResult, err := e.git(ctx, workspaceDir, "diff", "--cached", "--", path)
		if err != nil {
			return "", fmt.Errorf("failed to get staged diff for %q: %w", path, err)
		}
		unstagedResult, err := e.git(ctx, workspaceDir, "diff", "--", path)
		if err != nil {
			return "", fmt.Errorf("failed to get unstaged diff for %q: %w", path, err)
		}
		staged := strings.TrimSpace(string(stagedResult.Stdout))
		unstaged := strings.TrimSpace(string(unstagedResult.Stdout))
		if staged != "" && unstaged != "" {
			return staged + "\n\n" + unstaged, nil
		}
		if staged != "" {
			return staged, nil
		}
		return unstaged, nil
	default:
		if strings.TrimSpace(base) == "" {
			base = "HEAD"
		}
		args = []string{"diff", fmt.Sprintf("%s..%s", base, head), "--", path}
	}
	result, err := e.git(ctx, workspaceDir, args...)
	if err != nil {
		return "", fmt.Errorf("failed to get diff for %q: %w", path, err)
	}
	return strings.TrimSpace(string(result.Stdout)), nil
}

func dedupePaths(paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	result := make([]string, 0, len(paths))
	for _, path := range paths {
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		result = append(result, path)
	}
	return result
}

func formatCommandError(err error, result commandrunner.Result) error {
	stderr := strings.TrimSpace(string(result.Stderr))
	if stderr != "" {
		return fmt.Errorf("%w: %s", err, stderr)
	}

	stdout := strings.TrimSpace(string(result.Stdout))
	if stdout != "" {
		return fmt.Errorf("%w: %s", err, stdout)
	}

	return err
}

func isGitPathMissing(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "does not exist") {
		return true
	}
	if strings.Contains(message, "not in the index") {
		return true
	}
	if strings.Contains(message, "unknown revision or path not in the working tree") {
		return true
	}
	if strings.Contains(message, "ambiguous argument") {
		return true
	}
	if strings.Contains(message, "pathspec") && strings.Contains(message, "did not match") {
		return true
	}
	return false
}

func (e *HostCodeEnvironment) resolveDiffRefs(ctx context.Context, workspaceDir string, base string, head string) (string, string, string, error) {
	base = strings.TrimSpace(base)
	head = strings.TrimSpace(head)
	resolvedBase := base
	resolvedHead := head
	mergeBase := resolvedBase
	if isWorkspaceTokenRef(head) {
		return resolvedBase, resolvedHead, mergeBase, nil
	}

	if resolvedBase == "" {
		resolvedBase = "HEAD"
	}
	var resolveErr error
	resolvedBase, resolveErr = e.resolveRef(ctx, workspaceDir, resolvedBase)
	if resolveErr != nil {
		return "", "", "", resolveErr
	}
	resolvedHead, resolveErr = e.resolveRef(ctx, workspaceDir, resolvedHead)
	if resolveErr != nil {
		return "", "", "", resolveErr
	}

	mergeBase, resolveErr = e.mergeBase(ctx, workspaceDir, resolvedBase, resolvedHead)
	if resolveErr != nil {
		return "", "", "", resolveErr
	}

	return resolvedBase, resolvedHead, mergeBase, nil
}

func (e *HostCodeEnvironment) collectChangedPaths(ctx context.Context, workspaceDir string, head string, mergeBase string, resolvedHead string) ([]string, error) {
	switch head {
	case "", "@staged":
		return e.listChangedPaths(ctx, workspaceDir, "diff", "--cached", "--name-only", "--diff-filter=ACMRTUXB")
	case "@all":
		staged, err := e.listChangedPaths(ctx, workspaceDir, "diff", "--cached", "--name-only", "--diff-filter=ACMRTUXB")
		if err != nil {
			return nil, err
		}
		unstaged, err := e.listChangedPaths(ctx, workspaceDir, "diff", "--name-only", "--diff-filter=ACMRTUXB")
		if err != nil {
			return nil, err
		}
		untracked, err := e.listChangedPaths(ctx, workspaceDir, "ls-files", "--others", "--exclude-standard")
		if err != nil {
			return nil, err
		}
		return dedupePaths(append(append(staged, unstaged...), untracked...)), nil
	default:
		return e.listChangedPaths(ctx, workspaceDir, "diff", "--name-only", "--diff-filter=ACMRTUXB", fmt.Sprintf("%s..%s", mergeBase, resolvedHead))
	}
}

func (e *HostCodeEnvironment) git(ctx context.Context, workspaceDir string, args ...string) (commandrunner.Result, error) {
	result, err := e.runner.Run(ctx, "git", append([]string{"-C", workspaceDir}, args...)...)
	if err != nil {
		return result, formatCommandError(err, result)
	}
	return result, nil
}

func (e *HostCodeEnvironment) isShallowRepository(ctx context.Context, workspaceDir string) (bool, error) {
	result, err := e.git(ctx, workspaceDir, "rev-parse", "--is-shallow-repository")
	if err != nil {
		return false, err
	}
	value := strings.TrimSpace(string(result.Stdout))
	switch value {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf("unexpected shallow repository value %q", value)
	}
}

func newHostCodeEnvironmentTempDirMaker(userHomeDir func() (string, error)) func() (string, error) {
	return func() (string, error) {
		homeDir, err := userHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to resolve user home directory: %w", err)
		}
		homeDir = strings.TrimSpace(homeDir)
		if homeDir == "" {
			return "", fmt.Errorf("failed to resolve user home directory: empty path")
		}

		baseDir := filepath.Join(homeDir, hostCodeEnvironmentTempBaseDirName)
		if err := os.MkdirAll(baseDir, 0o700); err != nil {
			return "", fmt.Errorf("failed to create temporary workspace base directory %q: %w", baseDir, err)
		}
		if err := os.Chmod(baseDir, 0o700); err != nil {
			return "", fmt.Errorf("failed to secure temporary workspace base directory %q: %w", baseDir, err)
		}

		workspaceDir, err := os.MkdirTemp(baseDir, "bentos-coding-agent-*")
		if err != nil {
			return "", fmt.Errorf("failed to create temporary workspace directory under %q: %w", baseDir, err)
		}
		return workspaceDir, nil
	}
}

func (a *HostOpencodeAgent) resolveModelSpec(ctx context.Context, provider string, model string) (string, error) {
	provider = strings.TrimSpace(provider)
	model = strings.TrimSpace(model)

	if provider == "" {
		if model != "" {
			a.logger.Warnf("coding-agent opencode provider is empty; clearing model %q", model)
		}
		return "", nil
	}

	if model == "" {
		models, err := a.listOpencodeModels(ctx, provider)
		if err != nil {
			a.logger.Warnf("coding-agent opencode failed to list models for provider %s: %v", provider, err)
			return "", nil
		}
		if len(models) == 0 {
			a.logger.Warnf("coding-agent opencode no models returned for provider %s", provider)
			return "", nil
		}
		model = selectDefaultOpencodeModel(provider, models)
	}

	if model == "" {
		return "", nil
	}
	return provider + "/" + model, nil
}

func (a *HostOpencodeAgent) listOpencodeModels(ctx context.Context, provider string) ([]string, error) {
	result, err := a.runner.RunStream(ctx, nil, "opencode", "models", provider)
	if err != nil {
		return nil, formatCommandError(err, result)
	}
	return parseOpencodeModelList(provider, string(result.Stdout)), nil
}

func selectDefaultOpencodeModel(provider string, models []string) string {
	defaultModel, ok := defaultOpencodeModels[strings.ToLower(provider)]
	if ok {
		for _, candidate := range models {
			if strings.EqualFold(candidate, defaultModel) {
				return candidate
			}
		}
	}
	return models[0]
}

func parseOpencodeModelList(provider string, stdout string) []string {
	provider = strings.TrimSpace(provider)
	providerLower := strings.ToLower(provider)
	lines := strings.Split(stdout, "\n")
	models := make([]string, 0, len(lines))

	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		model := strings.TrimSpace(fields[0])
		if model == "" {
			continue
		}
		if strings.Contains(model, "/") {
			lower := strings.ToLower(model)
			prefix := providerLower + "/"
			if providerLower != "" && strings.HasPrefix(lower, prefix) {
				model = model[len(prefix):]
			} else {
				model = model[strings.LastIndex(model, "/")+1:]
			}
		}
		model = strings.TrimSpace(model)
		if model == "" {
			continue
		}
		models = append(models, model)
	}
	return models
}

var defaultOpencodeModels = map[string]string{
	"openai":    "gpt-5.3-codex",
	"anthropic": "claude-sonnet-4-6",
	"gemini":    "gemini-3-pro-preview",
	"google":    "gemini-3-pro-preview",
}

func (a *HostOpencodeAgent) ensureOpencodeInstalled(ctx context.Context) error {
	if a.installer == nil {
		a.installer = toolinstall.NewInstaller(toolinstall.Config{})
	}
	return a.installer.EnsureOpencodeInstalled(ctx)
}

func newOpencodeJSONStreamParser(logger usecase.Logger) *opencodeJSONStreamParser {
	if logger == nil {
		logger = stdlogger.Nop()
	}

	parser := &opencodeJSONStreamParser{
		logger: logger,
	}
	parser.stdoutLineBuffer = newLineBuffer(parser.consumeLine)
	return parser
}

func (p *opencodeJSONStreamParser) consumeLine(rawLine string) {
	if p.firstError != nil {
		return
	}
	p.lineNumber++

	line := strings.TrimSpace(rawLine)
	if line == "" {
		return
	}
	p.parsedLineCount++

	var event map[string]any
	if err := json.Unmarshal([]byte(line), &event); err != nil {
		p.firstError = fmt.Errorf("failed to parse opencode json output at line %d: %w", p.lineNumber, err)
		return
	}
	if sessionID, ok := event["sessionID"].(string); ok {
		sessionID = strings.TrimSpace(sessionID)
		if sessionID != "" {
			p.sessionID = sessionID
		}
	}

	parsedEvent := extractParsedOpencodeEvent(event)
	action := strings.TrimSpace(parsedEvent.Action)
	if action != "" && action != "agent started step" && !strings.HasPrefix(action, "agent finished step") {
		p.logger.Tracef("coding-agent trace action=%q line=%d", parsedEvent.Action, p.lineNumber)
	}

	candidate := parsedEvent.Text
	if strings.TrimSpace(candidate) == "" {
		return
	}

	if parsedEvent.Type == "assistant_delta" {
		p.assistantDeltaCount++
		p.assistantDelta.WriteString(candidate)
		p.logger.Tracef(
			"coding-agent trace action=%q line=%d index=%d chars=%d",
			"agent streamed assistant delta",
			p.lineNumber,
			p.assistantDeltaCount,
			len(candidate),
		)
		return
	}

	p.assistantMessageCount++
	p.finalText = candidate
	p.logger.Tracef(
		"coding-agent trace action=%q line=%d index=%d chars=%d",
		"agent produced assistant message",
		p.lineNumber,
		p.assistantMessageCount,
		len(candidate),
	)
}

func newLineBuffer(consumeLine func(string)) lineBuffer {
	return lineBuffer{
		consumeLine: consumeLine,
	}
}

func extractParsedOpencodeEvent(event map[string]any) parsedOpencodeEvent {
	eventType, _ := event["type"].(string)
	eventType = strings.ToLower(strings.TrimSpace(eventType))

	switch eventType {
	case "step_start":
		return parsedOpencodeEvent{
			Type:   eventType,
			Action: "agent started step",
		}
	case "step_finish":
		action := "agent finished step"
		if reason := extractStepFinishReason(event); reason != "" {
			action = fmt.Sprintf("agent finished step reason=%s", reason)
		}
		return parsedOpencodeEvent{
			Type:   eventType,
			Action: action,
		}
	case "tool_use":
		action := extractToolUseAction(event)
		if action == "" {
			action = "agent used tool"
		}
		return parsedOpencodeEvent{
			Type:   eventType,
			Action: action,
		}
	}

	if eventType == "text" {
		if part, ok := event["part"].(map[string]any); ok {
			partType, _ := part["type"].(string)
			if strings.EqualFold(strings.TrimSpace(partType), "text") {
				if text, _ := part["text"].(string); strings.TrimSpace(text) != "" {
					return parsedOpencodeEvent{
						Type:   "assistant_message",
						Text:   text,
						Action: "agent produced assistant message",
					}
				}
			}
		}
	}

	if role, _ := event["role"].(string); strings.EqualFold(strings.TrimSpace(role), "assistant") {
		if content := extractTextFromValue(event["content"]); strings.TrimSpace(content) != "" {
			return parsedOpencodeEvent{
				Type:   "assistant_message",
				Text:   content,
				Action: "agent produced assistant message",
			}
		}
		if text := extractTextFromValue(event["text"]); strings.TrimSpace(text) != "" {
			return parsedOpencodeEvent{
				Type:   "assistant_message",
				Text:   text,
				Action: "agent produced assistant message",
			}
		}
	}

	if message, ok := event["message"].(map[string]any); ok {
		if role, _ := message["role"].(string); strings.EqualFold(strings.TrimSpace(role), "assistant") {
			if content := extractTextFromValue(message["content"]); strings.TrimSpace(content) != "" {
				return parsedOpencodeEvent{
					Type:   "assistant_message",
					Text:   content,
					Action: "agent produced assistant message",
				}
			}
			if text := extractTextFromValue(message["text"]); strings.TrimSpace(text) != "" {
				return parsedOpencodeEvent{
					Type:   "assistant_message",
					Text:   text,
					Action: "agent produced assistant message",
				}
			}
		}
	}

	if strings.Contains(eventType, "assistant") && strings.Contains(eventType, "delta") {
		if delta, _ := event["delta"].(string); strings.TrimSpace(delta) != "" {
			return parsedOpencodeEvent{
				Type:   "assistant_delta",
				Text:   delta,
				Action: "agent streamed assistant delta",
			}
		}
	}

	return parsedOpencodeEvent{Type: "other"}
}

func extractStepFinishReason(event map[string]any) string {
	if reason, _ := event["reason"].(string); strings.TrimSpace(reason) != "" {
		return strings.TrimSpace(reason)
	}
	part, _ := event["part"].(map[string]any)
	if part == nil {
		return ""
	}
	if reason, _ := part["reason"].(string); strings.TrimSpace(reason) != "" {
		return strings.TrimSpace(reason)
	}
	return ""
}

func extractToolUseAction(event map[string]any) string {
	part, _ := event["part"].(map[string]any)
	if part == nil {
		return ""
	}

	toolName, _ := part["tool"].(string)
	toolName = strings.TrimSpace(toolName)
	if toolName == "" {
		return ""
	}

	input := map[string]any{}
	if state, ok := part["state"].(map[string]any); ok {
		if typedInput, ok := state["input"].(map[string]any); ok {
			input = typedInput
		}
	}

	filePath := extractFirstNonEmptyString(input, "filePath", "path", "filename", "file")
	command := extractFirstNonEmptyString(input, "command", "cmd", "script")
	command = truncateForTrace(command, 256)

	switch strings.ToLower(toolName) {
	case "read":
		if filePath != "" {
			return fmt.Sprintf("agent read file %s", filePath)
		}
		return "agent read file"
	case "edit", "write", "replace", "patch", "multi_edit":
		if filePath != "" {
			return fmt.Sprintf("agent edited file %s", filePath)
		}
		return "agent edited file"
	case "bash", "shell", "run", "command", "exec", "execute", "terminal":
		if command != "" {
			return fmt.Sprintf("agent ran command %q", command)
		}
		return "agent ran command"
	default:
		if filePath != "" {
			return fmt.Sprintf("agent used tool %s on file %s", toolName, filePath)
		}
		if command != "" {
			return fmt.Sprintf("agent used tool %s with command %q", toolName, command)
		}
		return fmt.Sprintf("agent used tool %s", toolName)
	}
}

func extractFirstNonEmptyString(source map[string]any, keys ...string) string {
	for _, key := range keys {
		value := extractTextFromValue(source[key])
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func extractTextFromValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			part := strings.TrimSpace(extractTextFromValue(item))
			if part != "" {
				parts = append(parts, part)
			}
		}
		return strings.Join(parts, "\n")
	case map[string]any:
		if text, _ := typed["text"].(string); strings.TrimSpace(text) != "" {
			return text
		}
		if content := extractTextFromValue(typed["content"]); strings.TrimSpace(content) != "" {
			return content
		}
		if delta, _ := typed["delta"].(string); strings.TrimSpace(delta) != "" {
			return delta
		}
	}
	return ""
}

func truncateForTrace(value string, maxChars int) string {
	if maxChars <= 0 {
		return ""
	}
	if len(value) <= maxChars {
		return value
	}
	return strings.TrimSpace(value[:maxChars]) + " [truncated ...]"
}
