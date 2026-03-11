package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

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

func (c *Client) buildRequestBody(params contracts.GenerateParams, responseFormat map[string]any) requestBody {
	messages := make([]map[string]any, 0, len(params.Messages)+1)
	if params.SystemPrompt != "" {
		messages = append(messages, map[string]any{
			"role":    "system",
			"content": params.SystemPrompt,
		})
	}
	for _, message := range params.Messages {
		messages = append(messages, map[string]any{
			"role":    "user",
			"content": message,
		})
	}

	return requestBody{
		Model:          c.config.Model,
		Messages:       messages,
		ResponseFormat: responseFormat,
	}
}

func (c *Client) generate(ctx context.Context, reqBody requestBody) (string, error) {
	raw, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	url := strings.TrimRight(c.config.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.config.APIKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("openai-compatible API returned status %d", resp.StatusCode)
	}

	var apiResp responseBody
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return "", err
	}
	if len(apiResp.Choices) == 0 {
		return "", fmt.Errorf("openai-compatible API returned no choices")
	}

	return apiResp.Choices[0].Message.Content, nil
}
