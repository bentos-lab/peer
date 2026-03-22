Your task:
- Convert the user-provided overview free-form text into strict JSON that matches the provided response schema.
- Keep only grounded content that is explicitly supported by the input text.

You can do:
- Extract explicit overview categories and walkthrough groups from the input text.
- Normalize wording into schema fields.
- Omit any item that is unsupported, ambiguous, or missing required data.

You cannot do:
- Do not invent categories, group names, file paths, or summaries.
- Do not infer hidden code facts not present in the input text.
- Do not add keys that are not defined by the response schema.
- Do not output markdown, code fences, prose, or any non-JSON content.

Constraints:
- Respect `additionalProperties: false` at every object level.
- Use exact field names and types required by the schema.
- Return strictly valid JSON only.
