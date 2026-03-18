# Review Spec

## Objective

Produce useful code review messages from changed content using an LLM.

## Inputs

`ReviewRequest` includes:
- review id
- repository
- change request number
- title/description
- refs and metadata

Changed contents are loaded by `ReviewInputProvider`.

## Rule Packs

Current version supports one hardcoded pack:
- `core/v1`
- policy source file: `usecase/rulepack/core_policy_v1.md` (embedded at build time via `go:embed`)

`core/v1` instruction focus:
- potential bugs
- risky logic changes
- maintainability/readability improvements
- test impact suggestions

Explicitly excluded:
- fixed Clean Architecture checklists
- fixed SOLID checklists

Policy authoring mode:
- runtime review policy is written as Markdown Go template (`*.md`)
- non-developers can modify policy text without changing Go code
- current template data includes: `ReviewLanguage`

## LLM Contract

Generic LLM generation contracts are defined at `usecase/contracts`:
- `Message`
- `GenerateParams`
- `LLMGenerator`

Current reviewer adapter (`adapter/outbound/reviewer/llm`) expects model output content as JSON:

```json
{
  "summary": "string",
  "findings": [
    {
      "filePath": "path/to/file.go",
      "line": 123,
      "severity": "CRITICAL|MAJOR|MINOR|NIT",
      "title": "Short finding title",
      "detail": "Why this matters",
      "suggestion": "Suggested improvement"
    }
  ]
}
```

## Output Strategy

For each review:
- emit grouped messages by file/area that has findings
- emit one summary message
- do not emit "good/no issue" comments for clean files

## Publishing Behavior

- GitHub/GitLab: publish comments/notes only
- CLI: print grouped messages + summary

No merge-blocking logic is implemented in current version.
