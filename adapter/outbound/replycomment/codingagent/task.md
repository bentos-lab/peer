Primary task:
- Answer the user question using the codebase context.
- Use merge-base diff between BASE and HEAD to ground your answer.
- Provide analysis only. Do not edit files.

Hard constraints:
- DO NOT edit files.
- DO NOT apply fixes.
- DO NOT run commands that mutate repository state.
- Analysis and reporting only.

Diff commands (required):
- Verify refs:
  - `git rev-parse --verify "{{ .Base }}^{commit}"`
  - `git rev-parse --verify "{{ .Head }}^{commit}"`
- Resolve merge-base:
  - `git merge-base "{{ .Base }}" "{{ .Head }}"`
- Inspect changed files:
  - `git diff --name-status "<merge-base>" "{{ .Head }}"`
- Inspect and anchor changed line ranges:
  - `git diff --unified=0 --no-color "<merge-base>" "{{ .Head }}"`

PR context:
- Repository: {{ .Repository }}
- Base: {{ .Base }}
- Head: {{ .Head }}
- Title: {{ .Title }}
- Description: {{ .Description }}

Thread history:
{{ .Thread }}

User question:
{{ .Question }}

{{- if .ExtraGuidance }}
Guidance:
{{ .ExtraGuidance }}
{{- end }}

Response guidance:
- Answer clearly and concisely.
- Reference relevant files/lines when available.
- If the question requests edits, suggest changes instead of editing.
- If details are missing, ask clarifying questions at the end.
