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
- Verify refs:
  - `git rev-parse --verify "{{ .Base }}^{commit}"`
  - `git rev-parse --verify "{{ .Head }}^{commit}"`
- Resolve merge-base:
  - `git merge-base "{{ .Base }}" "{{ .Head }}"`
- Inspect changed files:
  - `git diff --name-status "<merge-base>" "{{ .Head }}"`
- Inspect hunk-level changes:
  - `git diff --unified=0 --no-color "<merge-base>" "{{ .Head }}"`

Metadata:
- Repository: {{ .Repository }}
- Base: {{ .Base }}
- Head: {{ .Head }}
- Title: {{ .Title }}
- Description: {{ .Description }}

{{- if .ExtraGuidance }}
Guidance:
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
