package host

import (
	"context"
	"strings"

	"github.com/bentos-lab/peer/shared/toolinstall"
)

func (a *HostOpencodeAgent) resolveModelSpec(ctx context.Context, provider string, model string) (string, error) {
	provider = strings.TrimSpace(provider)
	model = strings.TrimSpace(model)

	if provider == "" {
		if model != "" {
			a.logger.Warnf("coding-agent opencode provider is empty; clearing model %q", model)
		}
		return "", nil
	}

	if model == "" {
		models, err := a.listOpencodeModels(ctx, provider)
		if err != nil {
			a.logger.Warnf("coding-agent opencode failed to list models for provider %s: %v", provider, err)
			return "", nil
		}
		if len(models) == 0 {
			a.logger.Warnf("coding-agent opencode no models returned for provider %s", provider)
			return "", nil
		}
		model = selectDefaultOpencodeModel(provider, models)
	}

	if model == "" {
		return "", nil
	}
	return provider + "/" + model, nil
}

func (a *HostOpencodeAgent) listOpencodeModels(ctx context.Context, provider string) ([]string, error) {
	result, err := a.runner.RunStream(ctx, nil, "opencode", "models", provider)
	if err != nil {
		return nil, formatCommandError(err, result)
	}
	return parseOpencodeModelList(provider, string(result.Stdout)), nil
}

func selectDefaultOpencodeModel(provider string, models []string) string {
	defaultModel, ok := defaultOpencodeModels[strings.ToLower(provider)]
	if ok {
		for _, candidate := range models {
			if strings.EqualFold(candidate, defaultModel) {
				return candidate
			}
		}
	}
	return models[0]
}

func parseOpencodeModelList(provider string, stdout string) []string {
	provider = strings.TrimSpace(provider)
	providerLower := strings.ToLower(provider)
	lines := strings.Split(stdout, "\n")
	models := make([]string, 0, len(lines))

	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		model := strings.TrimSpace(fields[0])
		if model == "" {
			continue
		}
		if strings.Contains(model, "/") {
			lower := strings.ToLower(model)
			prefix := providerLower + "/"
			if providerLower != "" && strings.HasPrefix(lower, prefix) {
				model = model[len(prefix):]
			} else {
				model = model[strings.LastIndex(model, "/")+1:]
			}
		}
		model = strings.TrimSpace(model)
		if model == "" {
			continue
		}
		models = append(models, model)
	}
	return models
}

var defaultOpencodeModels = map[string]string{
	"openai":    "gpt-5.3-codex",
	"anthropic": "claude-sonnet-4-6",
	"gemini":    "gemini-3-pro-preview",
	"google":    "gemini-3-pro-preview",
}

func (a *HostOpencodeAgent) ensureOpencodeInstalled(ctx context.Context) error {
	if a.installer == nil {
		a.installer = toolinstall.NewOpencodeInstaller(nil)
	}
	return a.installer.EnsureOpencodeInstalled(ctx)
}
