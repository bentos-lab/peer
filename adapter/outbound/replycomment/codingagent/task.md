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
{{- if eq .Head "@staged" }}
- Head uses staged workspace mode. You are only allow to compare staged changes.
- Inspect changed files:
  - `git diff --cached --name-status`
- Inspect and anchor changed line ranges:
  - `git diff --cached --unified=0 --no-color`
{{- else if eq .Head "@all" }}
- Head uses full workspace mode (staged + unstaged + untracked). You should compare all current changes with base.
- Inspect changed files:
  - `git diff --cached --name-status`
  - `git diff --name-status`
  - `git ls-files --others --exclude-standard`
- Inspect and anchor changed line ranges:
  - `git diff --cached --unified=0 --no-color`
  - `git diff --unified=0 --no-color`
{{- else if and .Base .Head }}
- Verify refs:
  - `git rev-parse --verify "{{ .Base }}^{commit}"`
  - `git rev-parse --verify "{{ .Head }}^{commit}"`
- Resolve merge-base:
  - `git merge-base "{{ .Base }}" "{{ .Head }}"`
- Inspect changed files:
  - `git diff --name-status "<merge-base>" "{{ .Head }}"`
- Inspect and anchor changed line ranges:
  - `git diff --unified=0 --no-color "<merge-base>" "{{ .Head }}"`
{{- else if .Head }}
- Base is empty; fallback to head-only inspection.
- Verify ref:
  - `git rev-parse --verify "{{ .Head }}^{commit}"`
- Inspect changed files:
  - `git show --name-status --no-color "{{ .Head }}"`
- Inspect and anchor changed line ranges:
  - `git show --unified=0 --no-color "{{ .Head }}"`
{{- else if .Base }}
- Head is empty; fallback to base-only inspection.
- Verify ref:
  - `git rev-parse --verify "{{ .Base }}^{commit}"`
- Inspect changed files:
  - `git show --name-status --no-color "{{ .Base }}"`
- Inspect and anchor changed line ranges:
  - `git show --unified=0 --no-color "{{ .Base }}"`
{{- else }}
- Base and Head are empty; fallback to workspace diff inspection.
- Inspect changed files:
  - `git status --short`
  - `git diff --name-status`
- Inspect and anchor changed line ranges:
  - `git diff --unified=0 --no-color`
{{- end }}
- If fallback mode is used due to missing Base/Head, mention limited confidence.

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

Response guidance:
- Answer clearly and concisely.
- Reference relevant files/lines when available.
- If the question requests edits, suggest changes instead of editing.
- If details are missing, ask clarifying questions at the end.
