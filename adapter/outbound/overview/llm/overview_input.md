Summarize the following code changes.

Return:
1. `categories`: high-level TL;DR by fixed categories that actually changed.
2. `walkthroughs`: grouped story of related file changes.

{{- if .Title }}
Change title: {{ .Title }}
{{- end }}
{{- if .Description }}
Change description:
{{ .Description }}
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
