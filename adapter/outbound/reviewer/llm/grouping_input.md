Group the following findings into fixable batches.

Maximum findings per group: {{ .MaxGroupSize }}

Candidates:
{{- range .Candidates }}

Finding key: {{ .Key }}
File: {{ .FilePath }}
Range: {{ .StartLine }}-{{ .EndLine }}
Severity: {{ .Severity }}
Title: {{ .Title }}
Detail: {{ .Detail }}
Relevant diff/context:
{{ .DiffSnippet }}
{{- end }}
