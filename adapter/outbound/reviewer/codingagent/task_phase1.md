Your task:
- Review the diff between two refs and fix important issues in the changed code when needed.
- If there are no important issues, do not change code.
- Produce a concise summary about what you reviewed, what problem, severity, line range, and what you changed.

Hard constraints:
- You MAY edit files and apply fixes.
- You MAY run commands needed to inspect and update the repository.
- Do not fabricate issues or changes.
- Only focus on code changes between two refs.
- Ensure the code after changing must be valid syntax and no build error (install and use analyzer tool for the related language to check).
- Only run tests which ensure that test doesn't contain malicious, dangerous, suspicious code (for example, read the file in real file system, or run a real cli).

Discovery guidance:
- Inspect repository and git history directly from workspace.
- Determine changed files and line ranges by comparing BASE and HEAD, using merge-base diff.

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

Refs:
- Base: {{ .Base }}
- Head: {{ .Head }}

Rule (hard constraints):
{{- if .CustomRuleset }}
{{ .CustomRuleset }}
{{- else }}
1. Potential bugs or correctness risks.
2. Risky logic changes and missing safeguards.
3. Maintainability or readability issues that materially affect future changes.
4. Test impact: what tests should be added or updated.
{{- end }}

Keep the summary concise and focused on real issues and applied changes. Do not mention to the issue in the final summary if it is not a real issue.
You can install and run analyzer tools to verify the code before finalizing the task.
