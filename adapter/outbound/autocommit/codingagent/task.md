Primary task:
- Analyze these local changes and produce a conventional commit message.

Hard constraints:
- DO NOT edit files.
- DO NOT apply fixes.
- DO NOT run commands that mutate repository state.
- Analysis and reporting only.

Discovery guidance:
- Inspect repository and git history directly from workspace.
- Use staged mode below to determine which diffs to inspect.

Diff commands (required):
{{- if .Staged }}
- Mode: staged changes only.
- Inspect changed files:
  - `git diff --cached --name-status`
- Inspect hunk-level changes:
  - `git diff --cached --unified=0 --no-color`
{{- else }}
- Mode: all changes (staged + unstaged + untracked).
- Inspect changed files:
  - `git diff --cached --name-status`
  - `git diff --name-status`
  - `git ls-files --others --exclude-standard`
- Inspect hunk-level changes:
  - `git diff --cached --unified=0 --no-color`
  - `git diff --unified=0 --no-color`
{{- end }}

Output guidance:
- Return plain text only. Which contain only commit message, not explanation, header.
- Output MUST be a conventional commit message.
- Use format: `type(scope): summary` or `type: summary`.
- Allowed types: feat, fix, docs, refactor, test, chore, build, ci, perf, style.
- Summary must be <= 72 characters.
- Optional body:
  - Each line <= 72 characters.
- Do not include quotes, backticks, or extra commentary.
