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
