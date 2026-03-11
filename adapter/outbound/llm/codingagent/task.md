Primary task:
- Produce the response by following the system prompt and user messages below.
- Use ONLY the data in the system prompt and user messages; do not invent extra context.

Hard constraints:
- DO NOT read files or repository contents.
- DO NOT change files.
- DO NOT run commands or tools.
- DO NOT browse/search the internet.
- Task execution and response only.

Input (combined into a single task):
System prompt:
{{- if .SystemPrompt }}
{{ .SystemPrompt }}
{{- else }}
(empty)
{{- end }}

User messages:
{{- if .Messages }}
{{- range $index, $message := .Messages }}
- {{ $message }}
{{- end }}
{{- else }}
(none)
{{- end }}

Output guidance:
- Return plain text only.
- Do not include JSON or markdown tables.
