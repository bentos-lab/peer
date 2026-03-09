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
      "startLine": 123,
      "endLine": 126,
      "severity": "CRITICAL|MAJOR|MINOR|NIT",
      "title": "Short finding title",
      "detail": "Why this matters",
      "suggestion": "Suggested improvement"
    }
  ]
}
```

Coding-agent reviewer adapter (`adapter/outbound/reviewer/codingagent`) follows the same JSON shape and additionally allows optional per-finding suggested change:

```json
{
  "summary": "string",
  "findings": [
    {
      "filePath": "path/to/file.go",
      "startLine": 123,
      "endLine": 126,
      "severity": "CRITICAL|MAJOR|MINOR|NIT",
      "title": "Short finding title",
      "detail": "Why this matters",
      "suggestion": "Suggested improvement",
      "suggestedChange": {
        "startLine": 123,
        "endLine": 126,
        "kind": "REPLACE|DELETE",
        "replacement": "string",
        "reason": "string"
      }
    }
  ]
}
```

Coding-agent review and overview prompts are split into two templates per flow:
- one `task.md` per flow (`reviewer/codingagent`, `overview/codingagent`) for coding-agent analysis instructions
- one `formatting_system.md` per flow (`reviewer/codingagent`, `overview/codingagent`) for free-text-to-JSON conversion contract
- metadata-only input (title, description, base/head, repo)
- no inline diff/file-content injection
- coding agent must discover context by inspecting repository/workspace directly

Overview adapter (`adapter/outbound/overview/llm`) uses an independent prompt and JSON contract:

```json
{
  "categories": [
    {
      "category": "Logic Updates|Refactoring|Security Fixes|Test Changes|Documentation|Infrastructure/Config",
      "summary": "string"
    }
  ],
  "walkthroughs": [
    {
      "groupName": "string",
      "files": ["path/to/file.go"],
      "summary": "string"
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
- When overview is enabled, generation and publishing run in one flow.
- GitHub overview comment is posted only on initial PR creation (`opened`).

No merge-blocking logic is implemented in current version.
