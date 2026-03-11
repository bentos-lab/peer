package wiring

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	codeenvhost "bentos-backend/adapter/outbound/codeenv/host"
	codingagent "bentos-backend/adapter/outbound/llm/codingagent"
	openai "bentos-backend/adapter/outbound/llm/openai"
	"bentos-backend/config"
	"bentos-backend/usecase"
	"bentos-backend/usecase/contracts"
)

const llmTimeout = 600 * time.Second

func buildOpenAIGenerator(selection LLMSelection) contracts.LLMGenerator {
	return openai.NewClient(&http.Client{Timeout: llmTimeout}, openai.ClientConfig{
		BaseURL: selection.OpenAI.BaseURL,
		APIKey:  selection.OpenAIAPIKey,
		Model:   selection.OpenAI.Model,
	})
}

func buildCodingAgentGenerator(cfg config.Config, logger usecase.Logger) (contracts.LLMGenerator, error) {
	agentName := strings.TrimSpace(strings.ToLower(cfg.CodingAgent.Agent))
	if agentName == "" {
		return nil, fmt.Errorf("coding agent is required")
	}
	if agentName != "opencode" {
		return nil, fmt.Errorf("unsupported coding agent for llm formatting: %s", agentName)
	}

	workspaceDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("resolve current workspace directory: %w", err)
	}

	agent := codeenvhost.NewHostOpencodeAgent(workspaceDir, nil, logger)
	return codingagent.NewGenerator(agent, codingagent.Config{
		Provider: strings.TrimSpace(cfg.CodingAgent.Provider),
		Model:    strings.TrimSpace(cfg.CodingAgent.Model),
	}, logger)
}
