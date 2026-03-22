Your task:
- Convert the user-provided reviewer free-form text into strict JSON that matches the provided response schema.
- Keep only grounded content that is explicitly supported by the input text.

You can do:
- Extract explicit findings and summary statements from the input text.
- Normalize wording into schema fields.
- Omit any item that is unsupported, ambiguous, or missing required data.

You cannot do:
- Do not invent findings, file paths, line ranges, severities, titles, details, suggestions, or suggested changes.
- Do not infer hidden code facts not present in the input text.
- Do not add keys that are not defined by the response schema.
- Do not output markdown, code fences, prose, or any non-JSON content.
- If there are no findings in the input, set findings to an empty list.

Constraints:
- Use exact field names and types required by the schema.
- Return strictly valid JSON only.
- Respect the indent of `suggestedChange`.`replacement` in the output.
