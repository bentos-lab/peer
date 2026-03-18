package gitlab

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	gitlabvcs "bentos-backend/adapter/outbound/vcs/gitlab"
)

// HookSyncer periodically ensures project hooks are installed.
type HookSyncer struct {
	client          HookClient
	logger          usecaseLogger
	webhookURL      string
	webhookSecret   string
	syncInterval    time.Duration
	statePath       string
	requiredHookCfg hookConfig
}

// HookClient defines required GitLab API client operations for hook sync.
type HookClient interface {
	ListUserEvents(ctx context.Context, after time.Time) ([]gitlabvcs.UserEvent, error)
	HasMaintainerAccess(ctx context.Context, projectID int) (bool, string, error)
	ListProjectHooks(ctx context.Context, projectID int) ([]gitlabvcs.ProjectHook, error)
	CreateProjectHook(ctx context.Context, projectID int, input gitlabvcs.HookInput) error
	UpdateProjectHook(ctx context.Context, projectID int, hookID int, input gitlabvcs.HookInput) error
}

// usecaseLogger matches the logging interface from usecase.
type usecaseLogger interface {
	Debugf(format string, args ...any)
	Infof(format string, args ...any)
	Warnf(format string, args ...any)
	Errorf(format string, args ...any)
}

type hookConfig struct {
	MergeRequestsEvents bool
	NoteEvents          bool
}

type syncState struct {
	LastCheck time.Time `json:"last_check"`
}

// NewHookSyncer creates a GitLab webhook syncer.
func NewHookSyncer(client HookClient, logger usecaseLogger, webhookURL string, webhookSecret string, syncInterval time.Duration, statePath string) *HookSyncer {
	return &HookSyncer{
		client:        client,
		logger:        logger,
		webhookURL:    strings.TrimSpace(webhookURL),
		webhookSecret: strings.TrimSpace(webhookSecret),
		syncInterval:  syncInterval,
		statePath:     strings.TrimSpace(statePath),
		requiredHookCfg: hookConfig{
			MergeRequestsEvents: true,
			NoteEvents:          true,
		},
	}
}

// Start runs periodic hook sync until ctx is cancelled.
func (s *HookSyncer) Start(ctx context.Context) {
	if s.client == nil {
		s.logger.Errorf("GitLab syncer is not configured; missing client.")
		return
	}
	if s.webhookURL == "" || s.webhookSecret == "" {
		s.logger.Errorf("GitLab syncer is not configured; missing webhook URL or secret.")
		return
	}
	if s.syncInterval <= 0 {
		s.syncInterval = 5 * time.Minute
	}
	if s.statePath == "" {
		s.statePath = "~/.peer/gitlab_sync.json"
	}

	s.runOnce(ctx)
	ticker := time.NewTicker(s.syncInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runOnce(ctx)
		}
	}
}

func (s *HookSyncer) runOnce(ctx context.Context) {
	state, err := s.loadState()
	if err != nil {
		s.logger.Warnf("GitLab syncer failed to load state: %v", err)
	}
	lastCheck := state.LastCheck
	events, err := s.client.ListUserEvents(ctx, lastCheck)
	if err != nil {
		s.logger.Errorf("GitLab syncer failed to load events: %v", err)
		return
	}

	now := time.Now().UTC()
	for _, event := range events {
		if event.ProjectID <= 0 {
			continue
		}
		if !lastCheck.IsZero() && !event.CreatedAt.After(lastCheck) {
			continue
		}

		s.logger.Infof("Handle Gitlab event: %s", event.ActionName)

		canManage, repository, err := s.client.HasMaintainerAccess(ctx, event.ProjectID)
		if err != nil {
			s.logger.Warnf("GitLab syncer failed to check access project=%d: %v", event.ProjectID, err)
			continue
		}
		if !canManage {
			s.logger.Debugf("GitLab syncer skipping project=%d (no maintainer access).", event.ProjectID)
			continue
		}
		if err := s.ensureHook(ctx, event.ProjectID, repository); err != nil {
			s.logger.Warnf("GitLab syncer failed to ensure hook project=%d: %v", event.ProjectID, err)
			continue
		}
	}

	state.LastCheck = now
	if err := s.saveState(state); err != nil {
		s.logger.Warnf("GitLab syncer failed to save state: %v", err)
	}
}

func (s *HookSyncer) ensureHook(ctx context.Context, projectID int, repository string) error {
	hooks, err := s.client.ListProjectHooks(ctx, projectID)
	if err != nil {
		return err
	}
	var existing *gitlabvcs.ProjectHook
	for i := range hooks {
		if strings.EqualFold(strings.TrimSpace(hooks[i].URL), s.webhookURL) {
			existing = &hooks[i]
			break
		}
	}

	input := gitlabvcs.HookInput{
		URL:                 s.webhookURL,
		Token:               s.webhookSecret,
		MergeRequestsEvents: s.requiredHookCfg.MergeRequestsEvents,
		NoteEvents:          s.requiredHookCfg.NoteEvents,
	}

	if existing == nil {
		s.logger.Infof("GitLab syncer creating hook repo=%q project=%d.", repository, projectID)
		return s.client.CreateProjectHook(ctx, projectID, input)
	}

	needsUpdate := !strings.EqualFold(strings.TrimSpace(existing.Token), s.webhookSecret) ||
		existing.MergeRequestsEvents != s.requiredHookCfg.MergeRequestsEvents ||
		existing.NoteEvents != s.requiredHookCfg.NoteEvents
	if !needsUpdate {
		return nil
	}
	s.logger.Infof("GitLab syncer updating hook repo=%q project=%d.", repository, projectID)
	return s.client.UpdateProjectHook(ctx, projectID, existing.ID, input)
}

func (s *HookSyncer) loadState() (syncState, error) {
	path, err := expandHomePath(s.statePath)
	if err != nil {
		return syncState{}, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return syncState{}, nil
		}
		return syncState{}, err
	}
	var state syncState
	if err := json.Unmarshal(raw, &state); err != nil {
		return syncState{}, err
	}
	return state, nil
}

func (s *HookSyncer) saveState(state syncState) error {
	path, err := expandHomePath(s.statePath)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	raw, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

func expandHomePath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", errors.New("path is required")
	}
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(home, strings.TrimPrefix(path, "~"))
	}
	return path, nil
}
