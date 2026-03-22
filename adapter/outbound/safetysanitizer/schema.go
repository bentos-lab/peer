package safetysanitizer

func sanitizerResponseSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"status", "sanitized_prompt", "refusal_message"},
		"properties": map[string]any{
			"status": map[string]any{
				"type": "string",
				"enum": []string{"ok", "unsupported", "unsafe"},
			},
			"sanitized_prompt": map[string]any{"type": "string"},
			"refusal_message":  map[string]any{"type": "string"},
		},
	}
}
