package codeagent

func issueAlignmentResponseSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"issue", "keyIdeas", "requirements"},
		"properties": map[string]any{
			"issue": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []string{"number"},
				"properties": map[string]any{
					"repository": map[string]any{"type": "string"},
					"number":     map[string]any{"type": "integer"},
					"title":      map[string]any{"type": "string"},
				},
			},
			"keyIdeas": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
			"requirements": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required":             []string{"requirement", "coverage"},
					"properties": map[string]any{
						"requirement": map[string]any{"type": "string"},
						"coverage":    map[string]any{"type": "string"},
					},
				},
			},
		},
	}
}

func issueKeyIdeasSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"keyIdeas"},
		"properties": map[string]any{
			"keyIdeas": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
		},
	}
}
