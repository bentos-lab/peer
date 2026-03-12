Primary task:
- Analyze this change request and summarize what changed.
- Focus on behavior impact, refactoring, tests, docs, security, and infrastructure/config updates.

Hard constraints:
- DO NOT edit files.
- DO NOT apply fixes.
- DO NOT run commands that mutate repository state.
- Analysis and reporting only.

Discovery guidance:
- Inspect repository and git history directly from workspace.
- Determine changed files and relationships by comparing BASE and HEAD, using merge-base diff.
- Use metadata below only as contextual hints.

Diff commands (required):
- Base and Head are the canonical comparison anchors; use both whenever available.
{{- if eq .Head "@staged" }}
- Head uses staged workspace mode. You are only allow to compare staged changes.
- Inspect changed files:
  - `git diff --cached --name-status`
- Inspect hunk-level changes:
  - `git diff --cached --unified=0 --no-color`
{{- else if eq .Head "@all" }}
- Head uses full workspace mode (staged + unstaged + untracked). You should compare all current changes with base.
- Inspect changed files:
  - `git diff --cached --name-status`
  - `git diff --name-status`
  - `git ls-files --others --exclude-standard`
- Inspect hunk-level changes:
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
- Inspect hunk-level changes:
  - `git diff --unified=0 --no-color "<merge-base>" "{{ .Head }}"`
{{- else if .Head }}
- Base is empty; fallback to head-only inspection.
- Verify ref:
  - `git rev-parse --verify "{{ .Head }}^{commit}"`
- Inspect changed files:
  - `git show --name-status --no-color "{{ .Head }}"`
- Inspect hunk-level changes:
  - `git show --unified=0 --no-color "{{ .Head }}"`
{{- else if .Base }}
- Head is empty; fallback to base-only inspection.
- Verify ref:
  - `git rev-parse --verify "{{ .Base }}^{commit}"`
- Inspect changed files:
  - `git show --name-status --no-color "{{ .Base }}"`
- Inspect hunk-level changes:
  - `git show --unified=0 --no-color "{{ .Base }}"`
{{- else }}
- Base and Head are empty; fallback to workspace diff inspection.
- Inspect changed files:
  - `git status --short`
  - `git diff --name-status`
- Inspect hunk-level changes:
  - `git diff --unified=0 --no-color`
{{- end }}
- If fallback mode is used due to missing Base/Head, mention limited confidence in `summary`.

Metadata:
- Repository: {{ .Repository }}
- Base: {{ .Base }}
- Head: {{ .Head }}
- Title: {{ .Title }}
- Description: {{ .Description }}

{{- if .ExtraGuidance }}
Custom recipe guidance:
{{ .ExtraGuidance }}

{{- end }}
Output guidance:
- Return plain text only.
- Do not output markdown tables or JSON.
- Use deterministic section order and stable labels exactly as below:
  - `summary`
  - `categories`
  - `walkthroughs`
- Do not add extra narrative outside these sections.

Required output content:
- `summary`:
  - Overall change intent.
  - Net impact on behavior and system quality.
- `categories`:
  - Behavior-focused items covering logic, refactor, security, tests, docs, and infrastructure/config.
  - For each item include:
    - impacted scope
    - why it matters
- `walkthroughs`:
  - One walkthrough block per change cluster using consistent labels.
  - Each block must include:
    - cluster name
    - file list
    - concise change explanation
    - observable impact
    - diff evidence cue (hunk-level location or changed symbol names)
