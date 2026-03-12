Primary task:
- Add missing tests, docs, and/or comments for the changed code.
- Scope ONLY to changes between BASE and HEAD using merge-base diff.

Hard constraints:
- DO NOT commit or push.
- DO NOT run commands that mutate git history.
- You MAY edit files to add tests/docs/comments.

Discovery guidance:
- Inspect repository and git history directly from workspace.
- Determine changed files and line ranges by comparing BASE and HEAD, using merge-base diff.
- Use metadata below only as contextual hints.

Diff commands (required):
- Base and Head are the canonical comparison anchors; use both whenever available.
{{- if eq .Head "@staged" }}
- Head uses staged workspace mode.
- Inspect changed files:
  - `git diff --cached --name-status`
- Inspect and anchor changed line ranges:
  - `git diff --cached --unified=0 --no-color`
{{- else if eq .Head "@all" }}
- Head uses full workspace mode (staged + unstaged + untracked).
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

Metadata:
- Repository: {{ .Repository }}
- Base: {{ .Base }}
- Head: {{ .Head }}
- Title: {{ .Title }}
- Description: {{ .Description }}
- Head branch (if PR): {{ .HeadBranch }}

{{- if .ExtraGuidance }}
Custom recipe guidance:
{{ .ExtraGuidance }}

{{- end }}
Output guidance:
- Make the changes directly in the repository.
- Keep changes minimal and scoped to the diff.
- Prefer deterministic formatting and stable file ordering.
- Do not print large diffs; brief status output is fine.

Output format (required):
- After finishing edits, output a PR-comment-ready report in this exact structure:
  - Line 1: `Autogen agent report`
  - Line 2: `Summary: <one sentence>`
  - Section `Added tests:` with `- <file>: <short note>` bullets (or `- none`)
  - Section `Added docs/comments:` with `- <file>: <short note>` bullets (or `- none`)
  - Section `Key files touched:` with `- <file>` bullets (or `- none`)
  - Section `Notes/limits:` with `- <note>` bullets (or `- none`)
- Keep the report concise and avoid copying diffs or large code blocks.

{{- if .Docs }}
Docs/Comments instructions:
- Add documentation and comments aligned with the repository's existing documentation style, conventions, and tone.
- Add documentation to existing documentation locations already used in the repo (for example existing files under docs/, README.md, or other established documentation files). Do not create new documentation files for autogen output.
- Documentation can be doc comments or existing documentation files, depending on repository conventions.
- Add comments wherever they improve maintainability (for example public functions/methods, magic numbers, complex logic, or other places needing clarity), not only for non-obvious logic.
- Keep docs/comments scoped to the diff.
- If no doc/comment is neccessary, just leave no change. You don't need to aggressive add docs/comment.
{{- else }}
Do not generate any docs or comment.
{{- end }}

{{- if .Tests }}
Tests instructions:
- Add or update tests aligned with the repository's existing test framework, naming conventions, and structure.
- Test public/exported APIs only; validate private logic through public APIs.
- Cover changed behavior and new edge cases introduced by the diff.
- Keep tests deterministic and minimal.
- If no test is neccessary, just leave no change. You don't need to aggressive add tests.
{{- else }}
Do not generate any tests.
{{- end }}
