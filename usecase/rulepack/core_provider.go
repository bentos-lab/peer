package rulepack

import (
	"bytes"
	"context"
	_ "embed"
	"strings"
	"text/template"

	"bentos-backend/usecase"
)

//go:embed core_policy_v1.md
var corePolicyTemplateRaw string

// CoreRulePackProvider returns the core v1 rule pack from embedded markdown template.
type CoreRulePackProvider struct{}

// NewCoreRulePackProvider creates a new CoreRulePackProvider.
func NewCoreRulePackProvider() *CoreRulePackProvider {
	return &CoreRulePackProvider{}
}

type corePolicyTemplateData struct {
	ReviewLanguage string
}

// CorePack returns the core v1 rule pack by rendering embedded markdown policy.
func (p *CoreRulePackProvider) CorePack(_ context.Context) (usecase.RulePack, error) {
	parsedTemplate, err := template.New("core_policy").Parse(corePolicyTemplateRaw)
	if err != nil {
		return usecase.RulePack{}, err
	}

	var rendered bytes.Buffer
	if err := parsedTemplate.Execute(&rendered, corePolicyTemplateData{
		ReviewLanguage: "English",
	}); err != nil {
		return usecase.RulePack{}, err
	}

	return usecase.RulePack{
		ID:      "core",
		Version: "v1",
		Name:    "Core Review Pack",
		Instructions: []string{
			strings.TrimSpace(rendered.String()),
		},
	}, nil
}
