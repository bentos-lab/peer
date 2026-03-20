Primary task:
- Analyze this change request and return concise findings in plain text.
- Ground all findings in real changed code between BASE and HEAD, using merge-base diff.

Hard constraints:
- DO NOT edit files.
- DO NOT apply fixes.
- DO NOT run commands that mutate repository state.
- Analysis and reporting only.

Discovery guidance:
- Inspect repository and git history directly from workspace.
- Determine changed files and line ranges by comparing BASE and HEAD, using merge-base diff.
- Use metadata below only as contextual hints.

Diff commands (required):
- Base and Head are the canonical comparison anchors; use both whenever available.
{{- if eq .Head "@staged" }}
- Head uses staged workspace mode. You are only allow to compare staged changes.
{{- if .Base }}
- Base is set; compare staged changes against Base.
- Inspect changed files:
  - `git diff --cached --name-status "{{ .Base }}"`
- Inspect and anchor changed line ranges:
  - `git diff --cached --unified=0 --no-color "{{ .Base }}"`
{{- else }}
- Inspect changed files:
  - `git diff --cached --name-status`
- Inspect and anchor changed line ranges:
  - `git diff --cached --unified=0 --no-color`
{{- end }}
{{- else if eq .Head "@all" }}
- Head uses full workspace mode (staged + unstaged + untracked). You should compare all current changes with base.
{{- if .Base }}
- Inspect changed files:
  - `git diff --cached --name-status "{{ .Base }}"`
  - `git diff --name-status "{{ .Base }}"`
  - `git ls-files --others --exclude-standard`
- Inspect and anchor changed line ranges:
  - `git diff --cached --unified=0 --no-color "{{ .Base }}"`
  - `git diff --unified=0 --no-color "{{ .Base }}"`
{{- else }}
- Inspect changed files:
  - `git diff --cached --name-status`
  - `git diff --name-status`
  - `git ls-files --others --exclude-standard`
- Inspect and anchor changed line ranges:
  - `git diff --cached --unified=0 --no-color`
  - `git diff --unified=0 --no-color`
{{- end }}
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
- Base and Head are empty; treat as full workspace mode with Base=`HEAD` and Head=`@all`.
- Inspect changed files:
  - `git diff --cached --name-status`
  - `git diff --name-status`
  - `git ls-files --others --exclude-standard`
- Inspect and anchor changed line ranges:
  - `git diff --cached --unified=0 --no-color`
  - `git diff --unified=0 --no-color`
{{- end }}

Metadata:
- Repository: {{ .Repository }}
- Base: {{ .Base }}
- Head: {{ .Head }}
- Title: {{ .Title }}
- Description: {{ .Description }}
- Language: {{ .Language }}
- Include Suggested Changes: {{ .Suggestions }}

Rule pack (hard constraints):
{{- if .CustomRuleset }}
{{ .CustomRuleset }}
{{- else }}
{{ .RulePackText }}
{{- end }}

Output guidance:
- Return plain text only.
- Do not output markdown tables or JSON.
- Use deterministic section order and stable labels exactly as below:
  - `summary`
  - `findings`
- Do not add extra narrative outside these sections.
- Do not group findings by file or category; output a direct finding list only.

Extremely important:
- If there are no issues with the changes, you can output only a summary. Do not try too hard to find issues.

Required output content:
- `summary`:
  - Concise risk-focused summary of the review result.
- `findings`:
  - One finding block per issue using consistent labels.
  - Each finding must include:
    - file path
    - changed-code line range (`start-end`)
    - severity
    - short title
    - detailed explanation (risk and why this is a problem)
    - diff-grounded evidence

{{- if .Suggestions }}
Suggested changes behavior:
- Include `suggested_change` for every finding.
- `suggested_change` must include:
  - line range (`start-end`) for the suggested change target
  - `kind`: `replace` or `delete`
  - `reason`
  - `replacement`: contains the FULL code (or comment) for replace the old code in `start`-`end` line range, including old lines if those lines don't need to be replaced. Do not include free text in this field. This field is required for `replace`, must be empty for `delete`.
- Suggested changes must be actionable and scoped to the finding line range.
{{- else }}
- NEVER suggest changes for any finding. Your task is JUST analyze and find them.
{{- end }}
