package toolinstall

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"bentos-backend/adapter/outbound/commandrunner"

	"github.com/stretchr/testify/require"
)

func TestUpdaterDownloadsAndReplacesBinary(t *testing.T) {
	tarBytes := buildTarGz(t, "peer", []byte("new-binary"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/releases/latest":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"tag_name":"v1.2.3"}`))
		case "/download/v1.2.3/peer-v1.2.3-linux-amd64.tar.gz":
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write(tarBytes)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	workDir := t.TempDir()
	execPath := filepath.Join(workDir, "peer")
	require.NoError(t, os.WriteFile(execPath, []byte("old-binary"), 0755))

	updater := NewUpdater(&UpdateDeps{
		Base: &Deps{
			PreferTTY: func() *bool { v := false; return &v }(),
		},
		HTTPClient:      server.Client(),
		ReleaseURL:      server.URL + "/releases/latest",
		DownloadBaseURL: server.URL + "/download/%s/%s",
		ExecPath:        func() (string, error) { return execPath, nil },
		GOOS:            "linux",
		GOARCH:          "amd64",
	})

	result, err := updater.Update(context.Background())
	require.NoError(t, err)
	require.Equal(t, "v1.2.3", result.Version)

	updated, err := os.ReadFile(execPath)
	require.NoError(t, err)
	require.Equal(t, "new-binary", string(updated))
}

func TestUpdaterUsesElevatedMoveWhenNotWritable(t *testing.T) {
	tarBytes := buildTarGz(t, "peer", []byte("new-binary"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/releases/latest":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"tag_name":"v9.9.9"}`))
		case "/download/v9.9.9/peer-v9.9.9-linux-amd64.tar.gz":
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write(tarBytes)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	dir := t.TempDir()
	execPath := filepath.Join(dir, "peer")
	require.NoError(t, os.WriteFile(execPath, []byte("old-binary"), 0755))
	require.NoError(t, os.Chmod(dir, 0555))
	t.Cleanup(func() {
		_ = os.Chmod(dir, 0755)
	})

	tempDownloadDir := t.TempDir()
	updatedSrcPath := filepath.Join(tempDownloadDir, "peer")

	runner := commandrunner.NewDummyCommandRunner()
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{Name: "sudo", Args: []string{"mv", updatedSrcPath, execPath}},
		Result:   commandrunner.Result{},
	})
	runner.Enqueue(commandrunner.CommandStep{
		Expected: commandrunner.CommandCall{Name: "sudo", Args: []string{"chmod", "+x", execPath}},
		Result:   commandrunner.Result{},
	})

	stdin := bytes.NewBufferString("y\n")
	updater := NewUpdater(&UpdateDeps{
		Base: &Deps{
			StreamRunner: runner,
			PreferTTY:    func() *bool { v := false; return &v }(),
			Stdin:        stdin,
			Stdout:       &bytes.Buffer{},
			Stderr:       &bytes.Buffer{},
			GOOS:         "linux",
			IsTerminal:   func() bool { return true },
		},
		HTTPClient:      server.Client(),
		ReleaseURL:      server.URL + "/releases/latest",
		DownloadBaseURL: server.URL + "/download/%s/%s",
		ExecPath:        func() (string, error) { return execPath, nil },
		TempDir:         func() (string, error) { return tempDownloadDir, nil },
		GOOS:            "linux",
		GOARCH:          "amd64",
	})

	_, err := updater.Update(context.Background())
	require.NoError(t, err)
	require.NoError(t, runner.VerifyDone())
}

func buildTarGz(t *testing.T, name string, body []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name: name,
		Mode: 0755,
		Size: int64(len(body)),
	}))
	_, err := tw.Write(body)
	require.NoError(t, err)
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	return buf.Bytes()
}

func TestUpdaterAssetNameErrorsOnUnsupportedArch(t *testing.T) {
	updater := NewUpdater(&UpdateDeps{
		GOOS:   "linux",
		GOARCH: "mips",
	})
	_, err := updater.assetName("v1.0.0")
	require.Error(t, err)
}

func TestUpdaterReturnsErrorWhenReleaseMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(server.Close)

	updater := NewUpdater(&UpdateDeps{
		HTTPClient: server.Client(),
		ReleaseURL: server.URL + "/releases/latest",
		GOOS:       "linux",
		GOARCH:     "amd64",
	})

	_, err := updater.Update(context.Background())
	require.Error(t, err)
}

func TestUpdaterReturnsErrorWhenNotConfigured(t *testing.T) {
	var updater *Updater
	_, err := updater.Update(context.Background())
	require.Error(t, err)
}

func TestUpdaterDownloadFailurePropagates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/releases/latest":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"tag_name":"v1.2.3"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	tempDir := t.TempDir()
	execPath := filepath.Join(tempDir, "peer")
	require.NoError(t, os.WriteFile(execPath, []byte("old"), 0755))

	updater := NewUpdater(&UpdateDeps{
		HTTPClient:      server.Client(),
		ReleaseURL:      server.URL + "/releases/latest",
		DownloadBaseURL: server.URL + "/download/%s/%s",
		ExecPath:        func() (string, error) { return execPath, nil },
		GOOS:            "linux",
		GOARCH:          "amd64",
	})

	_, err := updater.Update(context.Background())
	require.Error(t, err)
}

func TestUpdaterReturnsErrorWhenExecPathFails(t *testing.T) {
	updater := NewUpdater(&UpdateDeps{
		ExecPath: func() (string, error) { return "", errors.New("boom") },
		GOOS:     "linux",
		GOARCH:   "amd64",
	})

	_, err := updater.Update(context.Background())
	require.Error(t, err)
}
