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

## Review Rules

When `ReviewRuleset` is empty, reviewers apply a hardcoded default rule list:
- potential bugs or correctness risks
- risky logic changes and missing safeguards
- maintainability or readability issues that materially affect future changes
- test impact suggestions

When `ReviewRuleset` is provided, it fully replaces the default rule list.

## LLM Contract

Generic LLM generation contracts are defined at `usecase/contracts`:
- `GenerateParams`
- `LLMGenerator`

`GenerateParams` includes `SystemPrompt` plus a list of user message strings. JSON schema for structured outputs is passed as a separate argument to `GenerateJSON`.

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
- ignore NIT findings when building review output
- emit one summary message when there are remaining findings
- do not emit "good/no issue" comments for clean files
- when no non-NIT findings remain, publish nothing (including summary)

## Publishing Behavior

- GitHub/GitLab: publish comments/notes only
- CLI: print grouped messages + summary when findings remain
- When overview is enabled, generation and publishing run in one flow.
- GitHub overview comment is posted only on initial PR creation (`opened`).
- Suggested changes are enabled only when configured (see [Configuration](/docs/configuration.md#environment-variables)).

No merge-blocking logic is implemented in current version.
