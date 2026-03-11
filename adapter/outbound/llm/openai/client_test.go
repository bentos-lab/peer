package openai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"bentos-backend/usecase/contracts"
	"github.com/stretchr/testify/require"
)

func TestClient_Generate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		var payload map[string]any
		err = json.Unmarshal(body, &payload)
		require.NoError(t, err)
		_, hasResponseFormat := payload["response_format"]
		require.False(t, hasResponseFormat)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"choices": [{
				"message": {
					"content": "done"
				}
			}]
		}`))
	}))
	defer server.Close()

	client := NewClient(server.Client(), ClientConfig{
		BaseURL: server.URL,
		APIKey:  "x",
		Model:   "gpt-test",
	})

	result, err := client.Generate(context.Background(), contracts.GenerateParams{
		SystemPrompt: "system",
		Messages:     []string{"hello"},
	})
	require.NoError(t, err)
	require.Equal(t, "done", result)
}

func TestClient_Generate_ReturnsStatusError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	client := NewClient(server.Client(), ClientConfig{
		BaseURL: server.URL,
		APIKey:  "x",
		Model:   "gpt-test",
	})

	_, err := client.Generate(context.Background(), contracts.GenerateParams{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "returned status")
}

func TestClient_Generate_ReturnsNoChoicesError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[]}`))
	}))
	defer server.Close()

	client := NewClient(server.Client(), ClientConfig{
		BaseURL: server.URL,
		APIKey:  "x",
		Model:   "gpt-test",
	})

	_, err := client.Generate(context.Background(), contracts.GenerateParams{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "no choices")
}

func TestClient_Generate_ReturnsDecodeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{not-json`))
	}))
	defer server.Close()

	client := NewClient(server.Client(), ClientConfig{
		BaseURL: server.URL,
		APIKey:  "x",
		Model:   "gpt-test",
	})

	_, err := client.Generate(context.Background(), contracts.GenerateParams{})
	require.Error(t, err)
}

func TestClient_GenerateJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		var payload map[string]any
		err = json.Unmarshal(body, &payload)
		require.NoError(t, err)

		responseFormatRaw, ok := payload["response_format"]
		require.True(t, ok)
		responseFormat, ok := responseFormatRaw.(map[string]any)
		require.True(t, ok)
		require.Equal(t, "json_object", responseFormat["type"])

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"choices": [{
				"message": {
					"content": "{\"summary\":\"done\"}"
				}
			}]
		}`))
	}))
	defer server.Close()

	client := NewClient(server.Client(), ClientConfig{
		BaseURL: server.URL,
		APIKey:  "x",
		Model:   "gpt-test",
	})

	result, err := client.GenerateJSON(context.Background(), contracts.GenerateParams{}, nil)
	require.NoError(t, err)
	require.Equal(t, "done", result["summary"])
}

func TestClient_GenerateJSON_UsesResponseSchema(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		var payload map[string]any
		err = json.Unmarshal(body, &payload)
		require.NoError(t, err)

		responseFormatRaw, ok := payload["response_format"]
		require.True(t, ok)
		responseFormat, ok := responseFormatRaw.(map[string]any)
		require.True(t, ok)
		require.Equal(t, "json_schema", responseFormat["type"])

		schemaWrapperRaw, ok := responseFormat["json_schema"]
		require.True(t, ok)
		schemaWrapper, ok := schemaWrapperRaw.(map[string]any)
		require.True(t, ok)
		require.Equal(t, "structured_output", schemaWrapper["name"])
		require.Equal(t, true, schemaWrapper["strict"])

		schemaRaw, ok := schemaWrapper["schema"]
		require.True(t, ok)
		schema, ok := schemaRaw.(map[string]any)
		require.True(t, ok)
		require.Equal(t, "object", schema["type"])

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"choices": [{
				"message": {
					"content": "{\"summary\":\"done\"}"
				}
			}]
		}`))
	}))
	defer server.Close()

	client := NewClient(server.Client(), ClientConfig{
		BaseURL: server.URL,
		APIKey:  "x",
		Model:   "gpt-test",
	})

	result, err := client.GenerateJSON(context.Background(), contracts.GenerateParams{}, map[string]any{
		"type": "object",
		"properties": map[string]any{
			"summary": map[string]any{"type": "string"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "done", result["summary"])
}

func TestClient_GenerateJSON_ReturnsDecodeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"choices": [{
				"message": {
					"content": "not-json"
				}
			}]
		}`))
	}))
	defer server.Close()

	client := NewClient(server.Client(), ClientConfig{
		BaseURL: server.URL,
		APIKey:  "x",
		Model:   "gpt-test",
	})

	_, err := client.GenerateJSON(context.Background(), contracts.GenerateParams{}, nil)
	require.Error(t, err)
}
