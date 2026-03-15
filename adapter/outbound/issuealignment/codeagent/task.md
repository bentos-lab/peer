Primary task:
- Evaluate how well the change request aligns to the linked issue requirements.
- Use the key ideas below as the true requirements baseline.
- Prefer evidence from changed files when assessing coverage.

Hard constraints:
- DO NOT edit files.
- DO NOT apply fixes.
- DO NOT propose code changes.
- Analysis and reporting only.

Metadata:
- Repository: {{ .Repository }}
- Base: {{ .Base }}
- Head: {{ .Head }}
- Title: {{ .Title }}
- Description: {{ .Description }}

Key ideas:
{{- range .KeyIdeas }}
- {{ . }}
{{- end }}

Output format (plain text only):
- Use stable section labels exactly as below:
  - `requirements`
- Do not output markdown tables or JSON.
- Only include requirements that are explicit in the key ideas list.
- Coverage must start with `COVERED`, `PARTIAL`, or `NOT COVERED`, followed by an explanatory sentence describing evidence or lack of evidence.

Example format:
requirements
- requirement: Requirement 1
  coverage: COVERED The diff updates the validation checks that the issue explicitly requires.
- requirement: Requirement 2
  coverage: NOT COVERED No changes reference this requirement in the diff, so coverage is not evident.
