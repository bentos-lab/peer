Generate suggested changes for this finding group.

Group ID: {{ .GroupID }}
Grouping rationale: {{ .Rationale }}

Group file diffs (only files in this group):
{{- range .GroupDiffs }}

File: {{ .FilePath }}
Diff:
{{ .DiffSnippet }}
{{- end }}

Findings in this group:
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
