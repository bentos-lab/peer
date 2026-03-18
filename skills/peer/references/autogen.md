# peer autogen

Purpose: Run autogen to generate tests/docs/comments for a change request or local diff.

## Flags

Core:
- `--vcs-provider` (default: auto-detect) provider name. Allowed values: `github`, `gitlab`, `gitlab:<host>`.
- `--repo` (default: empty) repository URL or `owner/repo`. Empty means use current workspace.
- `--change-request` (default: empty) pull request number (positive integer). Cannot be used with `--base` or `--head`.
- `--base` (default: `HEAD`) base ref for diff. Ignored when `--change-request` is set.
- `--head` (default: `@all` for local, `HEAD` for `--repo`) head ref or `@staged`/`@all`. Ignored when `--change-request` is set.
- `--publish` (default: `false`) post autogen summary and push changes to PR branch. Requires `--change-request`.
- `--docs` (default: unset; falls back to recipe/config, config default is `false`) generate docs and code comments.
- `--tests` (default: unset; falls back to recipe/config, config default is `false`) generate tests.

LLM selection (applies only to this subcommand):
- `--llm-openai-base-url` (default: empty) OpenAI-compatible base URL or shortcut. Allowed values:
  - Full `http(s)://.../v1` URL.
  - Shortcuts: `openai`, `gemini`, `anthropic`.
  - Empty means use coding-agent LLM.
- `--llm-openai-model` (default: empty; falls back to config/env or shortcut default) model override.
- `--llm-openai-api-key` (default: empty; falls back to config/env) API key. Required when OpenAI mode is selected.
- `--code-agent` (default: from config/env `CODING_AGENT_NAME`, default `opencode`) coding agent name. Allowed values: `opencode` (only supported agent).
- `--code-agent-provider` (default: from config/env) coding agent provider override. Allowed values: any non-empty string (provider-specific).
- `--code-agent-model` (default: from config/env) coding agent model override. Allowed values: any non-empty string (provider-specific).

Verbosity:
- `-v, --verbose` (default: info) increase log verbosity. `-v` = debug, `-vv` = trace.

## Defaults and behavior
- If `--vcs-provider` is not set, peer auto-detects it from `--repo` or `git config remote.origin.url`.
- If `--repo` is set and `--head` is `@staged`/`@all`, peer returns an error (workspace tokens require local mode).
- Base/head defaults:
  - Local mode (no `--repo`, no `--change-request`): `--base=HEAD`, `--head=@all`.
  - Repo mode (with `--repo`, no `--change-request`): `--base=HEAD`, `--head=HEAD`.
- OpenAI shortcuts default models:
  - `openai` -> `gpt-4.1-mini`
  - `gemini` -> `gemini-2.5-flash-lite`
  - `anthropic` -> `claude-3-5-haiku-latest`
- If `--llm-openai-base-url` is a full URL and no model is set via flag/config, the command fails.

## Examples

Autogen tests and docs for PR 123:
```bash
peer autogen --change-request 123 --docs --tests
```

Autogen with publish enabled:
```bash
peer autogen --change-request 123 --docs --tests --publish
```

Autogen for a specific repo by URL:
```bash
peer autogen --repo https://github.com/example/repo.git --change-request 123 --docs
```

Autogen local workspace diff (defaults to base `HEAD`, head `@all`):
```bash
peer autogen --docs
```

Autogen local staged diff only:
```bash
peer autogen --tests --head @staged
```

Use OpenAI shortcut with default model:
```bash
peer autogen --change-request 123 --docs --llm-openai-base-url openai --llm-openai-api-key "$OPENAI_API_KEY"
```

Use OpenAI full URL + model:
```bash
peer autogen --change-request 123 --docs \
  --llm-openai-base-url https://api.openai.com/v1 \
  --llm-openai-model gpt-4.1 \
  --llm-openai-api-key "$OPENAI_API_KEY"
```

Use coding-agent overrides:
```bash
peer autogen --change-request 123 --docs --code-agent opencode --code-agent-provider opencode --code-agent-model "model-x"
```

Increase verbosity:
```bash
peer autogen --change-request 123 --docs -v
peer autogen --change-request 123 --docs -vv
```
