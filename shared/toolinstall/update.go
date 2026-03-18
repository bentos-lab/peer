package toolinstall

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	defaultRepo    = "bentos-lab/peer"
	defaultAppName = "peer"
)

// UpdateDeps provides optional overrides for update behavior.
type UpdateDeps struct {
	Base            *Deps
	HTTPClient      *http.Client
	Repo            string
	AppName         string
	ReleaseURL      string
	DownloadBaseURL string
	ExecPath        func() (string, error)
	TempDir         func() (string, error)
	GOOS            string
	GOARCH          string
}

// UpdateResult captures the update outcome.
type UpdateResult struct {
	Version string
	Asset   string
	Path    string
}

// Updater downloads and installs the latest stable CLI release.
type Updater struct {
	base            baseInstaller
	httpClient      *http.Client
	repo            string
	appName         string
	releaseURL      string
	downloadBaseURL string
	execPath        func() (string, error)
	goos            string
	goarch          string
	tempDir         func() (string, error)
}

// NewUpdater creates a new updater with optional deps.
func NewUpdater(deps *UpdateDeps) *Updater {
	if deps == nil {
		deps = &UpdateDeps{}
	}
	baseDeps := deps.Base
	if baseDeps == nil {
		baseDeps = &Deps{}
	}
	if deps.GOOS != "" {
		baseDeps.GOOS = deps.GOOS
	}
	base := newBaseInstaller(baseDeps)

	repo := strings.TrimSpace(deps.Repo)
	if repo == "" {
		repo = defaultRepo
	}
	appName := strings.TrimSpace(deps.AppName)
	if appName == "" {
		appName = defaultAppName
	}

	releaseURL := strings.TrimSpace(deps.ReleaseURL)
	if releaseURL == "" {
		releaseURL = fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	}
	downloadBaseURL := strings.TrimSpace(deps.DownloadBaseURL)
	if downloadBaseURL == "" {
		downloadBaseURL = fmt.Sprintf("https://github.com/%s/releases/download/%%s/%%s", repo)
	}

	httpClient := deps.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	execPath := deps.ExecPath
	if execPath == nil {
		execPath = os.Executable
	}
	goos := deps.GOOS
	if goos == "" {
		goos = runtime.GOOS
	}
	goarch := deps.GOARCH
	if goarch == "" {
		goarch = runtime.GOARCH
	}
	tempDir := deps.TempDir
	if tempDir == nil {
		tempDir = func() (string, error) {
			return os.MkdirTemp("", "peer-update-")
		}
	}

	return &Updater{
		base:            base,
		httpClient:      httpClient,
		repo:            repo,
		appName:         appName,
		releaseURL:      releaseURL,
		downloadBaseURL: downloadBaseURL,
		execPath:        execPath,
		goos:            goos,
		goarch:          goarch,
		tempDir:         tempDir,
	}
}

// Update downloads the latest stable release and replaces the current executable.
func (u *Updater) Update(ctx context.Context) (UpdateResult, error) {
	if u == nil {
		return UpdateResult{}, errors.New("update is not configured")
	}
	version, err := u.resolveLatestVersion(ctx)
	if err != nil {
		return UpdateResult{}, err
	}
	asset, err := u.assetName(version)
	if err != nil {
		return UpdateResult{}, err
	}
	execPath, err := u.execPath()
	if err != nil {
		return UpdateResult{}, err
	}
	tmpDir, err := u.tempDir()
	if err != nil {
		return UpdateResult{}, err
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	archivePath := filepath.Join(tmpDir, asset)
	downloadURL := fmt.Sprintf(u.downloadBaseURL, version, asset)
	if err := u.downloadFile(ctx, downloadURL, archivePath); err != nil {
		return UpdateResult{}, err
	}
	extractedPath, err := u.extractArchive(archivePath, tmpDir)
	if err != nil {
		return UpdateResult{}, err
	}
	if u.goos != "windows" {
		_ = os.Chmod(extractedPath, 0755)
	}
	if err := u.replaceBinary(ctx, extractedPath, execPath); err != nil {
		return UpdateResult{}, err
	}

	return UpdateResult{
		Version: version,
		Asset:   asset,
		Path:    execPath,
	}, nil
}

type releaseInfo struct {
	TagName string `json:"tag_name"`
}

func (u *Updater) resolveLatestVersion(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.releaseURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "peer-update")
	resp, err := u.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("failed to fetch latest release: %s", resp.Status)
	}
	var info releaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return "", err
	}
	version := strings.TrimSpace(info.TagName)
	if version == "" {
		return "", errors.New("latest release tag is empty")
	}
	return version, nil
}

func (u *Updater) assetName(version string) (string, error) {
	osName := strings.ToLower(u.goos)
	switch osName {
	case "linux", "darwin", "windows":
	default:
		return "", fmt.Errorf("unsupported os %q", u.goos)
	}

	arch := strings.ToLower(u.goarch)
	switch arch {
	case "amd64", "arm64":
	default:
		return "", fmt.Errorf("unsupported arch %q", u.goarch)
	}

	if osName == "windows" {
		return fmt.Sprintf("%s-%s-%s-%s.zip", u.appName, version, osName, arch), nil
	}
	return fmt.Sprintf("%s-%s-%s-%s.tar.gz", u.appName, version, osName, arch), nil
}

func (u *Updater) downloadFile(ctx context.Context, url string, destPath string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "peer-update")
	resp, err := u.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download failed: %s", resp.Status)
	}
	file, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()
	_, err = io.Copy(file, resp.Body)
	return err
}

func (u *Updater) extractArchive(archivePath string, destDir string) (string, error) {
	if strings.HasSuffix(strings.ToLower(archivePath), ".zip") {
		return u.extractZip(archivePath, destDir)
	}
	return u.extractTarGz(archivePath, destDir)
}

func (u *Updater) extractTarGz(archivePath string, destDir string) (string, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = file.Close()
	}()
	reader, err := gzip.NewReader(file)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = reader.Close()
	}()
	tarReader := tar.NewReader(reader)
	expected := u.binaryName()
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", err
		}
		if header.FileInfo().IsDir() {
			continue
		}
		if filepath.Base(header.Name) != expected {
			continue
		}
		outPath := filepath.Join(destDir, expected)
		out, err := os.Create(outPath)
		if err != nil {
			return "", err
		}
		if _, err := io.Copy(out, tarReader); err != nil {
			_ = out.Close()
			return "", err
		}
		if err := out.Close(); err != nil {
			return "", err
		}
		return outPath, nil
	}
	return "", fmt.Errorf("binary %q not found in archive", expected)
}

func (u *Updater) extractZip(archivePath string, destDir string) (string, error) {
	zipReader, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = zipReader.Close()
	}()
	expected := u.binaryName()
	for _, file := range zipReader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		if filepath.Base(file.Name) != expected {
			continue
		}
		in, err := file.Open()
		if err != nil {
			return "", err
		}
		outPath := filepath.Join(destDir, expected)
		out, err := os.Create(outPath)
		if err != nil {
			_ = in.Close()
			return "", err
		}
		if _, err := io.Copy(out, in); err != nil {
			_ = in.Close()
			_ = out.Close()
			return "", err
		}
		_ = in.Close()
		if err := out.Close(); err != nil {
			return "", err
		}
		return outPath, nil
	}
	return "", fmt.Errorf("binary %q not found in archive", expected)
}

func (u *Updater) replaceBinary(ctx context.Context, src string, dst string) error {
	dir := filepath.Dir(dst)
	canWrite, err := u.canWriteDir(dir)
	if err != nil {
		return err
	}
	if canWrite {
		return u.replaceInDir(src, dst, dir)
	}

	if !u.base.isTerminal() {
		u.printManualInstructions(src, dst)
		return errors.New("update requires elevated permissions")
	}
	ok, err := u.base.promptYesNo("Binary directory is not writable. Use elevated permissions to update? [Y/n]: ")
	if err != nil {
		return err
	}
	if !ok {
		u.printManualInstructions(src, dst)
		return errors.New("update canceled")
	}

	if u.goos == "windows" {
		return u.runElevatedMoveWindows(ctx, src, dst)
	}
	return u.runElevatedMoveUnix(ctx, src, dst)
}

func (u *Updater) canWriteDir(dir string) (bool, error) {
	file, err := os.CreateTemp(dir, ".peer-write-")
	if err != nil {
		if os.IsPermission(err) {
			return false, nil
		}
		return false, err
	}
	name := file.Name()
	_ = file.Close()
	_ = os.Remove(name)
	return true, nil
}

func (u *Updater) replaceInDir(src string, dst string, dir string) error {
	tempFile, err := os.CreateTemp(dir, u.appName+"-update-")
	if err != nil {
		if os.IsPermission(err) {
			return errors.New("destination directory is not writable")
		}
		return err
	}
	tempPath := tempFile.Name()
	if err := copyFile(src, tempFile); err != nil {
		_ = tempFile.Close()
		_ = os.Remove(tempPath)
		return err
	}
	if err := tempFile.Close(); err != nil {
		_ = os.Remove(tempPath)
		return err
	}
	if u.goos != "windows" {
		_ = os.Chmod(tempPath, 0755)
	}
	if u.goos == "windows" {
		if err := os.Remove(dst); err != nil && !errors.Is(err, os.ErrNotExist) {
			_ = os.Remove(tempPath)
			return err
		}
	}
	if err := os.Rename(tempPath, dst); err != nil {
		_ = os.Remove(tempPath)
		return err
	}
	return nil
}

func copyFile(src string, dst *os.File) error {
	file, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()
	_, err = io.Copy(dst, file)
	return err
}

func (u *Updater) runElevatedMoveUnix(ctx context.Context, src string, dst string) error {
	if err := u.base.run(ctx, "sudo", "mv", src, dst); err != nil {
		return err
	}
	return u.base.run(ctx, "sudo", "chmod", "+x", dst)
}

func (u *Updater) runElevatedMoveWindows(ctx context.Context, src string, dst string) error {
	escapedSrc := escapePowerShellString(src)
	escapedDst := escapePowerShellString(dst)
	moveCmd := fmt.Sprintf("Move-Item -Force -Path '%s' -Destination '%s'", escapedSrc, escapedDst)
	argumentList := strconv.Quote("-NoProfile -Command \"" + moveCmd + "\"")
	command := fmt.Sprintf("Start-Process -Verb RunAs -FilePath PowerShell -ArgumentList %s", argumentList)
	return u.base.run(ctx, "powershell", "-NoProfile", "-Command", command)
}

func escapePowerShellString(value string) string {
	return strings.ReplaceAll(value, "'", "''")
}

func (u *Updater) binaryName() string {
	if u.goos == "windows" {
		return u.appName + ".exe"
	}
	return u.appName
}

func (u *Updater) printManualInstructions(src string, dst string) {
	if u.goos == "windows" {
		_, _ = fmt.Fprintln(u.base.stderr, "Manual action required: run PowerShell as Administrator and execute:")
		_, _ = fmt.Fprintf(u.base.stderr, "Move-Item -Force -Path '%s' -Destination '%s'\n", src, dst)
		return
	}
	_, _ = fmt.Fprintln(u.base.stderr, "Manual action required:")
	_, _ = fmt.Fprintf(u.base.stderr, "sudo mv '%s' '%s'\n", src, dst)
	_, _ = fmt.Fprintf(u.base.stderr, "sudo chmod +x '%s'\n", dst)
}
