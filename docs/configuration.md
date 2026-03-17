# Configuration

This document describes runtime configuration, environment variables, and default behavior for review, overview, autogen, and replycomment.

## Precedence

Highest to lowest precedence:

1. CLI flags
2. `.autogit/config.toml`
3. Environment variables
4. Hard defaults

When a key is present in `config.toml`, it overrides environment defaults even if empty or false.

## Environment Variables

General:

- `LOG_LEVEL` (default: `info`)
- `PORT` (default: `8080`)
- `MAX_JOB_WORKERS` (default: `3`)

OpenAI-compatible formatter/sanitizer:

- `LLM_OPENAI_BASE_URL` (default: empty)
- `LLM_OPENAI_API_KEY` (default: empty)
- `LLM_OPENAI_MODEL` (default: empty)

Coding agent:

- `CODING_AGENT_NAME` (default: `opencode`)
- `CODING_AGENT_PROVIDER` (default: empty)
- `CODING_AGENT_MODEL` (default: empty)

GitHub App webhook:

- `GITHUB_WEBHOOK_SECRET` (required for webhook server)
- `GITHUB_APP_ID` (required for webhook server)
- `GITHUB_APP_PRIVATE_KEY` (required for webhook server)
- `GITHUB_API_BASE_URL` (default: `https://api.github.com`)

GitLab webhook:

- `GITLAB_TOKEN` (required for GitLab webhook server)
- `GITLAB_WEBHOOK_SECRET` (required for GitLab webhook server)
- `GITLAB_WEBHOOK_URL` (required for GitLab webhook sync)
- `GITLAB_API_BASE_URL` (default: `https://{GITLAB_HOST}/api/v4`)
- `GITLAB_SYNC_INTERVAL_MINUTES` (default: `5`)
- `GITLAB_SYNC_STATE_PATH` (default: `~/.autogit/gitlab_sync.json`)
- `GITLAB_HOST` (default: `gitlab.com`, only used when `GITLAB_API_BASE_URL` is empty)

Review:

- `REVIEW` (default: `true`)
- `REVIEW_SUGGESTED_CHANGES` (default: `false`)
- `REVIEW_EVENTS` (default: `opened,synchronize,reopened`)

Overview:

- `OVERVIEW` (default: `true`)
- `OVERVIEW_EVENTS` (default: `opened`)
- `OVERVIEW_ISSUE_ALIGNMENT` (default: `true`)

Replycomment:

- `REPLYCOMMENT` (default: `true`)
- `REPLYCOMMENT_EVENTS` (default: `issue_comment,pull_request_review_comment`)
- `REPLYCOMMENT_ACTIONS` (default: `created`)
- `REPLYCOMMENT_TRIGGER_NAME` (default: `autogitbot`)

Autogen:

- `AUTOGEN` (default: `false`)
- `AUTOGEN_EVENTS` (default: `opened,reopened,synchronize`)
- `AUTOGEN_DOCS` (default: `false`)
- `AUTOGEN_TESTS` (default: `false`)

Notes:

- Boolean env values follow Go `strconv.ParseBool` rules; invalid values fall back to defaults.
- List envs are comma-separated and normalized to lowercase.
- CLI flags can override review/overview/autogen settings for a single run.

## Default Behavior

- Review runs on `opened`, `synchronize`, and `reopened` by default.
- Overview runs on `opened` by default, and issue alignment is enabled by default.
- Replycomment listens for `issue_comment` and `pull_request_review_comment` with `created` action and a `@NAME` or `/NAME` trigger.
- Autogen is disabled by default; when enabled it runs on `opened`, `reopened`, `synchronize` and requires docs or tests to be enabled.
