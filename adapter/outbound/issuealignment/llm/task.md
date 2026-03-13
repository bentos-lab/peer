Primary task:
- Evaluate how well the change request aligns to the linked issue requirements.
- Use the key ideas below as the true requirements baseline.
- Prefer evidence from changed files when assessing coverage.

Hard constraints:
- DO NOT edit files.
- DO NOT apply fixes.
- Analysis and reporting only.

Metadata:
- Repository: {{ .Repository }}
- Base: {{ .Base }}
- Head: {{ .Head }}
- Title: {{ .Title }}
- Description: {{ .Description }}

Key ideas (true requirements):
{{- range .KeyIdeas }}
- {{ . }}
{{- end }}

Issue candidates:
{{- range .Issues }}
- {{ if .Repository }}{{ .Repository }}{{ end }}#{{ .Number }}: {{ .Title }}
  Body: {{ .Body }}
  {{- if .Comments }}
  Comments:
  {{- range .Comments }}
  - {{ .Author.Login }}: {{ .Body }}
  {{- end }}
  {{- end }}
{{- end }}

{{- if .Files }}
Changed files and contents:
{{- range .Files }}

File: {{ .Path }}
Changed content:
{{ .ChangedText }}
{{- end }}
{{- else }}
No changed file content was provided.
{{- end }}

{{- if .ExtraGuidance }}
Custom recipe guidance:
{{ .ExtraGuidance }}
{{- end }}

Output format (plain text only):
- Use stable section labels exactly as below:
  - `issue`
  - `key_ideas`
  - `requirements`
- Do not output markdown tables or JSON.
- Choose the single issue that best represents the key ideas.
- Only include requirements that are explicit in the key ideas list.
- Coverage should be one of: Yes, Partial, No, or Unknown.

Example format:
issue
- repository: owner/repo
- number: 123
- title: Issue title

key_ideas
- Requirement 1
- Requirement 2

requirements
- requirement: Requirement 1
  coverage: Yes
- requirement: Requirement 2
  coverage: Partial
