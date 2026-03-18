package skillupdate

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bentos-lab/peer/skills"
)

const peerSkillDirName = "peer"

var errUpdateNotConfigured = errors.New("skill update is not configured")

// Result captures the update outcome for a single target path.
type Result struct {
	Path string
	Err  error
}

// UpdateDeps provides optional overrides for skill update behavior.
type UpdateDeps struct {
	HomeDir        string
	Cwd            string
	XDGConfigHome  string
	XDGConfigDirs  []string
	ResolveGitRoot func(string) (string, error)
	SourceFS       fs.FS
	SourceRoot     string
}

// Updater updates peer skills using an embedded skill payload.
type Updater struct {
	deps UpdateDeps
}

// NewUpdater creates a new skill updater with optional dependencies.
func NewUpdater(deps *UpdateDeps) *Updater {
	defaults := defaultUpdateDeps()
	if deps == nil {
		return &Updater{deps: defaults}
	}
	if deps.HomeDir != "" {
		defaults.HomeDir = deps.HomeDir
	}
	if deps.Cwd != "" {
		defaults.Cwd = deps.Cwd
	}
	if deps.XDGConfigHome != "" {
		defaults.XDGConfigHome = deps.XDGConfigHome
	}
	if len(deps.XDGConfigDirs) > 0 {
		defaults.XDGConfigDirs = append([]string{}, deps.XDGConfigDirs...)
	}
	if deps.ResolveGitRoot != nil {
		defaults.ResolveGitRoot = deps.ResolveGitRoot
	}
	if deps.SourceFS != nil {
		defaults.SourceFS = deps.SourceFS
	}
	if deps.SourceRoot != "" {
		defaults.SourceRoot = deps.SourceRoot
	}
	return &Updater{deps: defaults}
}

// Update replaces target peer skills with embedded content.
func (u *Updater) Update(ctx context.Context, paths []string) ([]Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	deps, err := u.resolveDeps()
	if err != nil {
		return nil, err
	}
	targets, err := resolveTargets(paths, deps)
	if err != nil {
		return nil, err
	}
	results := make([]Result, 0, len(targets))
	var combined error
	for _, target := range targets {
		updateErr := updateTarget(deps.SourceFS, deps.SourceRoot, target)
		if updateErr != nil {
			combined = errors.Join(combined, fmt.Errorf("%s: %w", target, updateErr))
		}
		results = append(results, Result{Path: target, Err: updateErr})
	}
	return results, combined
}

func (u *Updater) resolveDeps() (UpdateDeps, error) {
	if u == nil {
		return UpdateDeps{}, errUpdateNotConfigured
	}
	deps := u.deps
	if deps.SourceFS == nil || deps.SourceRoot == "" {
		fsValue, root := skills.PeerSkillFS()
		deps.SourceFS = fsValue
		deps.SourceRoot = root
	}
	if deps.ResolveGitRoot == nil {
		deps.ResolveGitRoot = resolveGitRoot
	}
	if deps.Cwd == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return UpdateDeps{}, err
		}
		deps.Cwd = cwd
	}
	if deps.HomeDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return UpdateDeps{}, err
		}
		deps.HomeDir = home
	}
	if deps.XDGConfigHome == "" {
		deps.XDGConfigHome = os.Getenv("XDG_CONFIG_HOME")
	}
	if len(deps.XDGConfigDirs) == 0 {
		envValue := os.Getenv("XDG_CONFIG_DIRS")
		if envValue != "" {
			deps.XDGConfigDirs = strings.Split(envValue, ":")
		}
	}
	if deps.XDGConfigHome == "" && deps.HomeDir != "" {
		deps.XDGConfigHome = filepath.Join(deps.HomeDir, ".config")
	}
	if len(deps.XDGConfigDirs) == 0 {
		deps.XDGConfigDirs = []string{"/etc/xdg"}
	}
	return deps, nil
}

func defaultUpdateDeps() UpdateDeps {
	return UpdateDeps{}
}

func resolveTargets(paths []string, deps UpdateDeps) ([]string, error) {
	if len(paths) == 0 {
		return discoverTargets(deps)
	}
	return resolveTargetsFromPaths(paths, deps.HomeDir)
}

func resolveTargetsFromPaths(paths []string, home string) ([]string, error) {
	seen := map[string]struct{}{}
	var targets []string
	for _, raw := range paths {
		expanded, err := expandHome(raw, home)
		if err != nil {
			return nil, err
		}
		expanded = filepath.Clean(expanded)
		target, err := resolveTargetPath(expanded)
		if err != nil {
			return nil, err
		}
		if _, exists := seen[target]; exists {
			continue
		}
		seen[target] = struct{}{}
		targets = append(targets, target)
	}
	return targets, nil
}

func resolveTargetPath(pathValue string) (string, error) {
	if hasPeerSkill(pathValue) {
		return pathValue, nil
	}
	peerPath := filepath.Join(pathValue, peerSkillDirName)
	if hasPeerSkill(peerPath) {
		return peerPath, nil
	}
	return "", fmt.Errorf("invalid skill path: %s", pathValue)
}

func discoverTargets(deps UpdateDeps) ([]string, error) {
	rootDirs := []string{
		filepath.Join(".agents", "skills"),
		filepath.Join(".claude", "skills"),
		filepath.Join(".codex", "skills"),
		filepath.Join(".cursor", "skills"),
		filepath.Join(".windsurf", "skills"),
	}

	projectBases := projectBaseDirs(deps.Cwd, deps.ResolveGitRoot)
	userBases := []string{}
	if deps.HomeDir != "" {
		userBases = append(userBases, deps.HomeDir)
	}

	var xdgBases []string
	if deps.XDGConfigHome != "" {
		xdgBases = append(xdgBases, deps.XDGConfigHome)
	}
	for _, dir := range deps.XDGConfigDirs {
		if strings.TrimSpace(dir) == "" {
			continue
		}
		xdgBases = append(xdgBases, dir)
	}

	seen := map[string]struct{}{}
	var targets []string
	addTarget := func(target string) {
		if _, exists := seen[target]; exists {
			return
		}
		seen[target] = struct{}{}
		targets = append(targets, target)
	}

	scopes := [][]string{projectBases, userBases, xdgBases}
	for _, bases := range scopes {
		for _, base := range bases {
			for _, root := range rootDirs {
				candidateRoot := filepath.Join(base, root)
				peerPath := filepath.Join(candidateRoot, peerSkillDirName)
				if hasPeerSkill(peerPath) {
					addTarget(peerPath)
				}
			}
		}
	}

	return targets, nil
}

func projectBaseDirs(cwd string, resolveGitRoot func(string) (string, error)) []string {
	if cwd == "" {
		return nil
	}
	absCwd, err := filepath.Abs(cwd)
	if err != nil {
		return []string{cwd}
	}
	gitRoot := ""
	if resolveGitRoot != nil {
		if root, err := resolveGitRoot(absCwd); err == nil {
			gitRoot = root
		}
	}
	return ancestorDirs(absCwd, gitRoot)
}

func ancestorDirs(start string, stop string) []string {
	start = filepath.Clean(start)
	if start == "" {
		return nil
	}
	stop = filepath.Clean(stop)
	dirs := []string{}
	current := start
	for {
		dirs = append(dirs, current)
		if current == stop {
			break
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return dirs
}

func hasPeerSkill(dir string) bool {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return false
	}
	skillPath := filepath.Join(dir, "SKILL.md")
	info, err = os.Stat(skillPath)
	return err == nil && !info.IsDir()
}

func updateTarget(source fs.FS, sourceRoot string, target string) error {
	if source == nil {
		return errors.New("missing embedded skill filesystem")
	}
	if sourceRoot == "" {
		return errors.New("missing embedded skill root")
	}
	if err := removeDirContents(target); err != nil {
		return err
	}
	return copyFS(source, sourceRoot, target)
}

func removeDirContents(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		pathValue := filepath.Join(dir, entry.Name())
		if err := os.RemoveAll(pathValue); err != nil {
			return err
		}
	}
	return nil
}

func copyFS(source fs.FS, sourceRoot string, target string) error {
	return fs.WalkDir(source, sourceRoot, func(entryPath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(entryPath, sourceRoot)
		rel = strings.TrimPrefix(rel, "/")
		if rel == "" {
			return nil
		}
		targetPath := filepath.Join(target, filepath.FromSlash(rel))
		if d.IsDir() {
			return os.MkdirAll(targetPath, 0o755)
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		mode := info.Mode().Perm()
		if mode == 0 {
			mode = 0o644
		}
		file, err := source.Open(entryPath)
		if err != nil {
			return err
		}
		defer func() {
			_ = file.Close()
		}()
		return writeFileFromReader(targetPath, file, mode)
	})
}

func writeFileFromReader(pathValue string, reader io.Reader, mode fs.FileMode) error {
	targetFile, err := os.OpenFile(pathValue, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	defer func() {
		_ = targetFile.Close()
	}()
	_, err = io.Copy(targetFile, reader)
	return err
}

func expandHome(value string, home string) (string, error) {
	if value == "" {
		return "", errors.New("path is required")
	}
	if !strings.HasPrefix(value, "~") {
		return value, nil
	}
	if home == "" {
		return "", errors.New("cannot expand ~ without home directory")
	}
	if value == "~" {
		return home, nil
	}
	if strings.HasPrefix(value, "~/") {
		return filepath.Join(home, value[2:]), nil
	}
	return value, nil
}

func resolveGitRoot(cwd string) (string, error) {
	command := exec.Command("git", "-C", cwd, "rev-parse", "--show-toplevel")
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("resolve git root: %w", err)
	}
	root := strings.TrimSpace(string(output))
	if root == "" {
		return "", errors.New("resolve git root: empty output")
	}
	return root, nil
}
