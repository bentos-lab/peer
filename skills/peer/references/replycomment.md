# peer replycomment

Purpose: Reply to a specific PR comment with an LLM-generated response or answer a question locally.

## Flags

Core:
- `--vcs-provider` (default: auto-detect) provider name. Allowed values: `github`, `gitlab`, `gitlab:<host>`.
- `--repo` (default: empty) repository URL or `owner/repo`. Empty means use current workspace.
- `--change-request` (default: empty) pull request number (positive integer). Cannot be used with `--base` or `--head`.
- `--comment-id` (default: empty) comment id to answer.
- `--question` (default: empty) question text to answer locally (no publish).
- `--publish` (default: `false`) post reply as pull request comment. Requires `--comment-id` and `--change-request`.

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
- `-v, --verbose` (default: warning) increase log verbosity. `-v` = info, `-vv` = debug, `-vvv` = trace.

## Defaults and behavior
- If `--vcs-provider` is not set, peer auto-detects it from `--repo` or `git config remote.origin.url`.
- If `--repo` is set and `--head` is `@staged`/`@all`, peer returns an error (workspace tokens require local mode).
- Base/head defaults when running locally (no `--repo`, no `--change-request`): `--base=HEAD`, `--head=@all`.
- OpenAI shortcuts default models:
  - `openai` -> `gpt-4.1-mini`
  - `gemini` -> `gemini-2.5-flash-lite`
  - `anthropic` -> `claude-3-5-haiku-latest`
- If `--llm-openai-base-url` is a full URL and no model is set via flag/config, the command fails.

## Examples

Reply to comment in PR 123 and publish:
```bash
peer replycomment --change-request 123 --comment-id issuecomment-1234567890 --publish
```

Reply with explicit repo and provider:
```bash
peer replycomment --vcs-provider github --repo owner/repo --change-request 123 --comment-id issuecomment-1234567890 --publish
```

Answer a raw question locally (no publish):
```bash
peer replycomment --change-request 123 --question "What changed in the service layer?"
```

Use OpenAI shortcut with default model:
```bash
peer replycomment --change-request 123 --comment-id issuecomment-1234567890 --publish \
  --llm-openai-base-url openai --llm-openai-api-key "$OPENAI_API_KEY"
```

Use OpenAI full URL + model:
```bash
peer replycomment --change-request 123 --comment-id issuecomment-1234567890 --publish \
  --llm-openai-base-url https://api.openai.com/v1 \
  --llm-openai-model gpt-4.1 \
  --llm-openai-api-key "$OPENAI_API_KEY"
```

Use coding-agent overrides:
```bash
peer replycomment --change-request 123 --comment-id issuecomment-1234567890 --publish \
  --code-agent opencode --code-agent-provider opencode --code-agent-model "model-x"
```

Increase verbosity:
```bash
peer replycomment --change-request 123 --comment-id issuecomment-1234567890 --publish -v
peer replycomment --change-request 123 --comment-id issuecomment-1234567890 --publish -vv
```
