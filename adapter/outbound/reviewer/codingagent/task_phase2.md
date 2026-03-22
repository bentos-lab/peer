Based on the real issues you found and the changed code, construct a structured report.

Report output format:
- Return plain text only.
- Do not output markdown tables or JSON.
- Use deterministic section order and stable labels exactly as below:
  - `summary`
  - `findings`
- Do not add extra narrative outside these sections.

Important:
- If there are no issues with the changes, output only a summary and no findings. Do not fabricate or force issues.

Required output content must contain:
- `summary`:
  - Concise, risk-focused summary of the review result.
- `findings`:
  - One finding block per issue using consistent labels.
  - Each finding must include:
    - file path
    - line range (`start-end`) of the old code that contains the problem
    - severity
    - short title
    - detailed explanation (risk and why this is a problem)

{{- if .Suggestions }}
Also include the changed code (`suggested_change`) for every finding:
- `suggested_change` must include:
  - line range (`start-end`) of the old code that was replaced by the current code. The line range must match the old code exactly so replacing that range with `replacement` yields valid code.
  - `kind`: `replace` or `delete`
  - `reason`
  - `replacement`: the current code (or comment) that already replaced the old code in the `start`-`end` range. Include unchanged old lines if they must remain. Do not include free text in this field. Required for `replace`; must be empty for `delete`.
{{- end }}
