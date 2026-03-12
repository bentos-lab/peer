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
{{- if .SystemPrompt }}
System prompt:
{{ .SystemPrompt }}
{{- end }}

{{- if .Messages }}
User messages:
{{- range $index, $message := .Messages }}
- {{ $message }}
{{- end }}
{{- end }}

{{- if .HasSchema }}
The JSON output must follow the rule of the following JSON schema strictly:
{{ .Schema }}
{{- end }}

Output guidance:
- Return JSON only (converted from user data, not JSON schema).
- Do not include any thinking or extra text before or after the JSON.
