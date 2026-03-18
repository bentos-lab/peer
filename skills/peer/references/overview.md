# peer overview

Purpose: Generate a high-level summary for a change request or local diff.

## Flags

Core:
- `--vcs-provider` (default: auto-detect) provider name. Allowed values: `github`, `gitlab`, `gitlab:<host>`.
- `--repo` (default: empty) repository URL or `owner/repo`. Empty means use current workspace.
- `--change-request` (default: empty) pull request number (positive integer). Cannot be used with `--base` or `--head`.
- `--base` (default: `HEAD`) base ref for diff. Ignored when `--change-request` is set.
- `--head` (default: `@all` for local, `HEAD` for `--repo`) head ref or `@staged`/`@all`. Ignored when `--change-request` is set.
- `--publish` (default: `false`) post overview result as pull request comments. Requires `--change-request`.
- `--issue-alignment` (default: unset; falls back to recipe/config, config default is `true`) enable issue alignment analysis for overview.

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

Overview PR 123 in the current repository:
```bash
peer overview --change-request 123
```

Overview PR 123 and publish comments:
```bash
peer overview --change-request 123 --publish
```

Overview using explicit VCS provider:
```bash
peer overview --vcs-provider github --change-request 123
```

Overview a specific repo by URL:
```bash
peer overview --repo https://github.com/example/repo.git --change-request 123
```

Overview local workspace diff (defaults to base `HEAD`, head `@all`):
```bash
peer overview
```

Overview local staged diff only:
```bash
peer overview --head @staged
```

Enable issue alignment explicitly:
```bash
peer overview --change-request 123 --issue-alignment
```

Use OpenAI shortcut with default model:
```bash
peer overview --change-request 123 --llm-openai-base-url openai --llm-openai-api-key "$OPENAI_API_KEY"
```

Use OpenAI full URL + model:
```bash
peer overview --change-request 123 \
  --llm-openai-base-url https://api.openai.com/v1 \
  --llm-openai-model gpt-4.1 \
  --llm-openai-api-key "$OPENAI_API_KEY"
```

Use coding-agent overrides:
```bash
peer overview --change-request 123 --code-agent opencode --code-agent-provider opencode --code-agent-model "model-x"
```

Increase verbosity:
```bash
peer overview --change-request 123 -v
peer overview --change-request 123 -vv
```
