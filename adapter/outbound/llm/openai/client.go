package openai

import (
	"context"
	"encoding/json"
	"net/http"

	"bentos-backend/usecase/contracts"
)

// HTTPClient abstracts HTTP calls for testing.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// ClientConfig defines OpenAI-compatible client config.
type ClientConfig struct {
	BaseURL string
	APIKey  string
	Model   string
}

// Client is an OpenAI-compatible LLM adapter.
type Client struct {
	httpClient HTTPClient
	config     ClientConfig
}

// NewClient creates an OpenAI-compatible LLM client.
func NewClient(httpClient HTTPClient, config ClientConfig) *Client {
	return &Client{
		httpClient: httpClient,
		config:     config,
	}
}

type requestBody struct {
	Model          string           `json:"model"`
	Messages       []map[string]any `json:"messages"`
	ResponseFormat map[string]any   `json:"response_format,omitempty"`
}

type responseBody struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// Generate calls the OpenAI-compatible API and returns one text completion.
func (c *Client) Generate(ctx context.Context, params contracts.GenerateParams) (string, error) {
	reqBody := c.buildRequestBody(params, nil)
	return c.generate(ctx, reqBody)
}

// GenerateJSON calls Generate and decodes JSON content.
func (c *Client) GenerateJSON(ctx context.Context, params contracts.GenerateParams, schema map[string]any) (map[string]any, error) {
	responseFormat := map[string]any{
		"type": "json_object",
	}
	if len(schema) > 0 {
		responseFormat = map[string]any{
			"type": "json_schema",
			"json_schema": map[string]any{
				"name":   "structured_output",
				"strict": true,
				"schema": schema,
			},
		}
	}

	content, err := c.generate(ctx, c.buildRequestBody(params, responseFormat))
	if err != nil {
		return nil, err
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(content), &decoded); err != nil {
		return nil, err
	}
	return decoded, nil
}
