package llm

import "github.com/bentos-lab/peer/domain"

func reviewResponseSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"summary", "findings"},
		"properties": map[string]any{
			"summary": map[string]any{
				"type": "string",
			},
			"findings": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required": []string{
						"filePath",
						"startLine",
						"endLine",
						"severity",
						"title",
						"detail",
						"suggestion",
					},
					"properties": map[string]any{
						"filePath": map[string]any{
							"type": "string",
						},
						"startLine": map[string]any{
							"type":    "integer",
							"minimum": 1,
						},
						"endLine": map[string]any{
							"type":    "integer",
							"minimum": 1,
						},
						"severity": map[string]any{
							"type": "string",
							"enum": []string{
								string(domain.FindingSeverityCritical),
								string(domain.FindingSeverityMajor),
								string(domain.FindingSeverityMinor),
								string(domain.FindingSeverityNit),
							},
						},
						"title": map[string]any{
							"type": "string",
						},
						"detail": map[string]any{
							"type": "string",
						},
						"suggestion": map[string]any{
							"type": "string",
						},
					},
				},
			},
		},
	}
}
