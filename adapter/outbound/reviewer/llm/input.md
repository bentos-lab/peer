Review the following code changes and report actionable findings only.

Review language: {{ .Language }}

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
