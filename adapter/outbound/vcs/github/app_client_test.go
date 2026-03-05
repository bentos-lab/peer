package github

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAppClient_GetPullRequestChangedFiles(t *testing.T) {
	privateKey := mustGeneratePrivateKeyPEM(t)
	var installationTokenCalls int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/app/installations/123/access_tokens":
			atomic.AddInt32(&installationTokenCalls, 1)
			require.Equal(t, http.MethodPost, r.Method)
			_, _ = w.Write([]byte(`{"token":"token-1","expires_at":"2099-01-01T00:00:00Z"}`))
		case "/repos/org/repo/pulls/7/files":
			require.Equal(t, "100", r.URL.Query().Get("per_page"))
			require.Equal(t, "1", r.URL.Query().Get("page"))
			_, _ = w.Write([]byte(`[
				{"filename":"a.go","patch":"@@ -1 +1 @@\n-old\n+new"},
				{"filename":"b.png"}
			]`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := NewAppClient(server.Client(), AppClientConfig{
		APIBaseURL: server.URL,
		AppID:      "12345",
		PrivateKey: privateKey,
	})
	require.NoError(t, err)

	ctx := WithInstallationID(context.Background(), "123")
	files, err := client.GetPullRequestChangedFiles(ctx, "org/repo", 7)
	require.NoError(t, err)
	require.Len(t, files, 1)
	require.Equal(t, "a.go", files[0].Path)
	require.EqualValues(t, 1, atomic.LoadInt32(&installationTokenCalls))
}

func TestAppClient_UsesTokenCacheByInstallationID(t *testing.T) {
	privateKey := mustGeneratePrivateKeyPEM(t)
	var installationTokenCalls int32
	var commentCalls int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/app/installations/321/access_tokens":
			atomic.AddInt32(&installationTokenCalls, 1)
			_, _ = w.Write([]byte(`{"token":"token-abc","expires_at":"2099-01-01T00:00:00Z"}`))
		case "/repos/org/repo/issues/5/comments":
			atomic.AddInt32(&commentCalls, 1)
			require.Equal(t, "Bearer token-abc", r.Header.Get("Authorization"))
			_, _ = w.Write([]byte(`{"id":1}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := NewAppClient(server.Client(), AppClientConfig{
		APIBaseURL: server.URL,
		AppID:      "12345",
		PrivateKey: privateKey,
	})
	require.NoError(t, err)

	ctx := WithInstallationID(context.Background(), "321")
	require.NoError(t, client.CreateComment(ctx, "org/repo", 5, "hello 1"))
	require.NoError(t, client.CreateComment(ctx, "org/repo", 5, "hello 2"))
	require.EqualValues(t, 1, atomic.LoadInt32(&installationTokenCalls))
	require.EqualValues(t, 2, atomic.LoadInt32(&commentCalls))
}

func TestAppClient_FailsWhenInstallationIDMissing(t *testing.T) {
	privateKey := mustGeneratePrivateKeyPEM(t)
	client, err := NewAppClient(http.DefaultClient, AppClientConfig{
		APIBaseURL: "https://api.github.com",
		AppID:      "12345",
		PrivateKey: privateKey,
	})
	require.NoError(t, err)

	err = client.CreateComment(context.Background(), "org/repo", 1, "hello")
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing github app installation id")
}

func TestAppClient_CreateReviewCommentClassifiesInvalidAnchor(t *testing.T) {
	privateKey := mustGeneratePrivateKeyPEM(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/app/installations/42/access_tokens":
			_, _ = w.Write([]byte(`{"token":"token-1","expires_at":"2099-01-01T00:00:00Z"}`))
		case "/repos/org/repo/pulls/7":
			_, _ = w.Write([]byte(`{"head":{"sha":"abc123"}}`))
		case "/repos/org/repo/pulls/7/comments":
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"message":"line must be part of the diff"}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := NewAppClient(server.Client(), AppClientConfig{
		APIBaseURL: server.URL,
		AppID:      "12345",
		PrivateKey: privateKey,
	})
	require.NoError(t, err)

	err = client.CreateReviewComment(WithInstallationID(context.Background(), "42"), "org/repo", 7, CreateReviewCommentInput{
		Body:      "bad",
		Path:      "a.go",
		StartLine: 90,
		EndLine:   90,
	})
	require.Error(t, err)
	require.True(t, IsInvalidAnchorError(err))
}

func TestNewAppClient_RejectsInvalidPrivateKey(t *testing.T) {
	_, err := NewAppClient(http.DefaultClient, AppClientConfig{
		APIBaseURL: "https://api.github.com",
		AppID:      "12345",
		PrivateKey: "not-a-key",
	})
	require.Error(t, err)
}

func TestNewAppClient_AcceptsInlinePrivateKey(t *testing.T) {
	_, err := NewAppClient(http.DefaultClient, AppClientConfig{
		APIBaseURL: "https://api.github.com",
		AppID:      "12345",
		PrivateKey: mustGeneratePrivateKeyPEM(t),
	})
	require.NoError(t, err)
}

func TestNewAppClient_AcceptsPrivateKeyPath(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "app.pem")
	require.NoError(t, os.WriteFile(keyPath, []byte(mustGeneratePrivateKeyPEM(t)), 0o600))

	_, err := NewAppClient(http.DefaultClient, AppClientConfig{
		APIBaseURL: "https://api.github.com",
		AppID:      "12345",
		PrivateKey: keyPath,
	})
	require.NoError(t, err)
}

func TestNewAppClient_FailsFastWhenPrivateKeyPathReadFails(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "blocked.pem")
	require.NoError(t, os.WriteFile(keyPath, []byte(mustGeneratePrivateKeyPEM(t)), 0o600))
	require.NoError(t, os.Chmod(keyPath, 0o000))
	t.Cleanup(func() {
		_ = os.Chmod(keyPath, 0o600)
	})

	_, err := NewAppClient(http.DefaultClient, AppClientConfig{
		APIBaseURL: "https://api.github.com",
		AppID:      "12345",
		PrivateKey: keyPath,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to read github app private key file")
}

func TestNewAppClient_RejectsInvalidPrivateKeyFromPath(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "invalid.pem")
	require.NoError(t, os.WriteFile(keyPath, []byte("not-a-pem"), 0o600))

	_, err := NewAppClient(http.DefaultClient, AppClientConfig{
		APIBaseURL: "https://api.github.com",
		AppID:      "12345",
		PrivateKey: keyPath,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to decode github app private key PEM")
}

func TestNewAppClient_NonExistentPathFallsBackToInlineParsing(t *testing.T) {
	_, err := NewAppClient(http.DefaultClient, AppClientConfig{
		APIBaseURL: "https://api.github.com",
		AppID:      "12345",
		PrivateKey: "tmp/does/not/exist.pem",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to decode github app private key PEM")
}

func TestAppClient_RefreshesExpiredToken(t *testing.T) {
	privateKey := mustGeneratePrivateKeyPEM(t)
	var installationTokenCalls int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/app/installations/11/access_tokens":
			callNo := atomic.AddInt32(&installationTokenCalls, 1)
			token := fmt.Sprintf("token-%d", callNo)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"token":"%s","expires_at":"2099-01-01T00:00:00Z"}`, token)))
		case "/repos/org/repo/issues/9/comments":
			_, _ = w.Write([]byte(`{"id":1}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := NewAppClient(server.Client(), AppClientConfig{
		APIBaseURL: server.URL,
		AppID:      "12345",
		PrivateKey: privateKey,
	})
	require.NoError(t, err)
	client.now = func() time.Time {
		return time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	client.tokenByInstID["11"] = installationTokenCacheItem{
		Token:     "expired-token",
		ExpiresAt: time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	err = client.CreateComment(WithInstallationID(context.Background(), "11"), "org/repo", 9, "hello")
	require.NoError(t, err)
	require.EqualValues(t, 1, atomic.LoadInt32(&installationTokenCalls))
}

func mustGeneratePrivateKeyPEM(t *testing.T) string {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	encoded := x509.MarshalPKCS1PrivateKey(privateKey)
	block := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: encoded}
	return string(pem.EncodeToMemory(block))
}

func TestIsInvalidAnchorAPIError(t *testing.T) {
	err := errors.New("github API request failed with status 422: line must be part of the diff")
	require.True(t, isInvalidAnchorAPIError(err))
}
