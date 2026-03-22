package codingagent

import "github.com/bentos-lab/peer/domain"

func reviewResponseSchema() map[string]any {
	suggestedChangeSchema := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"startLine", "endLine", "kind", "replacement", "reason"},
		"properties": map[string]any{
			"startLine":   map[string]any{"type": "integer", "minimum": 1},
			"endLine":     map[string]any{"type": "integer", "minimum": 1},
			"kind":        map[string]any{"type": "string", "enum": []string{string(domain.SuggestedChangeKindReplace), string(domain.SuggestedChangeKindDelete)}},
			"replacement": map[string]any{"type": "string"},
			"reason":      map[string]any{"type": "string"},
		},
	}

	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"summary", "findings"},
		"properties": map[string]any{
			"summary": map[string]any{"type": "string"},
			"findings": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required":             []string{"filePath", "startLine", "endLine", "severity", "title", "detail", "suggestion"},
					"properties": map[string]any{
						"filePath":        map[string]any{"type": "string"},
						"startLine":       map[string]any{"type": "integer", "minimum": 1},
						"endLine":         map[string]any{"type": "integer", "minimum": 1},
						"severity":        map[string]any{"type": "string", "enum": []string{string(domain.FindingSeverityCritical), string(domain.FindingSeverityMajor), string(domain.FindingSeverityMinor), string(domain.FindingSeverityNit)}},
						"title":           map[string]any{"type": "string"},
						"detail":          map[string]any{"type": "string"},
						"suggestion":      map[string]any{"type": "string"},
						"suggestedChange": map[string]any{"anyOf": []any{suggestedChangeSchema, map[string]any{"type": "null"}}},
					},
				},
			},
		},
	}
}
