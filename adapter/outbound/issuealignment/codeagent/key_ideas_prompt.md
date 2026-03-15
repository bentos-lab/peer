Extract the main, true requirements from the issue contents below.
- Merge duplicates and keep only distinct requirements.
- Prefer explicit requirements stated in the issue text.
- Keep the list concise.

Issues:
{{- range .Issues }}
- {{ if .Repository }}{{ .Repository }}{{ end }}#{{ .Number }}: {{ .Title }}
  {{- if .Body }}
  Body: {{ .Body }}
  {{- end }}
  {{- if .Comments }}
  Comments:
  {{- range .Comments }}
  - {{ .Author.Login }}: {{ .Body }}
  {{- end }}
  {{- end }}
{{- end }}
