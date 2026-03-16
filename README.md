# bentos-backend

LLM-based pull/merge request reviewer built with Clean Architecture.

## Supported Flows

- GitHub webhook flow (`/webhook/github`) using GitHub App authentication.
- GitLab webhook flow (`/webhook/gitlab`).
- CLI flow for local/GitHub PR review.

## Prerequisites

- Go `1.26`

## Install Dependencies

```bash
go mod tidy
```

## Run Tests

```bash
go test ./...
```

## Environment Variables

Full list with defaults: [Configuration](/docs/configuration.md#environment-variables).

LLM_OPENAI (formatter + sanitizer only when enabled):

- `LLM_OPENAI_BASE_URL` (empty uses coding-agent LLM; shortcuts: `gemini`, `openai`, `anthropic`)
- `LLM_OPENAI_API_KEY` (required when `LLM_OPENAI_BASE_URL` is set)
- `LLM_OPENAI_MODEL` (optional; defaults depend on shortcut)

CODE AGENT:

- `CODING_AGENT_NAME` (default: `opencode`)
- `CODING_AGENT_PROVIDER` (optional, passed to opencode)
- `CODING_AGENT_MODEL` (optional, passed to opencode)

Feature: Core

- `LOG_LEVEL` (default: `info`)
- `OVERVIEW` (default: `true`)

Feature: Server (webhook-only)

- `PORT` (default: `8080`)

Feature: GitHub webhook

- `GITHUB_WEBHOOK_SECRET` (required)
- `GITHUB_APP_ID` (required)
- `GITHUB_APP_PRIVATE_KEY` (required, PEM content or path to PEM file; escaped `\n` is supported for inline mode)
- `GITHUB_API_BASE_URL` (optional, default: `https://api.github.com`)

Example: Inline PEM mode:

`GITHUB_APP_PRIVATE_KEY=-----BEGIN PRIVATE KEY-----\n...\n-----END PRIVATE KEY-----`

Example: File path mode:

`GITHUB_APP_PRIVATE_KEY=/run/secrets/github_app_private_key.pem`

`.env` is auto-loaded when present in current working directory.

## GitHub App Setup

1. Create a GitHub App.
2. Configure permissions:
- `Contents`: `Read and write` (autogen publish needs push access)
- `Pull requests`: `Read and write`
- `Issues`: `Read and write` (overview and reply comments)
- `Metadata`: `Read-only`
3. Subscribe to webhook event: `Pull requests`.
4. Set webhook URL to your server endpoint:
- `POST https://<your-host>/webhook/github`
5. Set webhook secret and use the same value for `GITHUB_WEBHOOK_SECRET`.
6. Generate and download the app private key.
7. Set env vars:
- `GITHUB_APP_ID`
- `GITHUB_APP_PRIVATE_KEY`
- `GITHUB_WEBHOOK_SECRET`
8. Install the GitHub App on target org/repositories.

## Run Autogit

Webhook server:

```bash
go run ./cmd/autogit webhook --vcs-provider github
```

Verbose logging (optional `-v`/`-vv`/`-vvv`):

```bash
go run ./cmd/autogit webhook --vcs-provider github -vv
```

Webhook routes:

- `POST /webhook/github`

GitHub PR actions that trigger review:

- `opened`
- `synchronize`
- `reopened`

For each trigger, the service:

1. Loads changed files from GitHub.
2. Runs overview when enabled (default action `opened`) and posts one overview comment.
3. Runs LLM review and posts inline review comments.
4. When both overview and review are enabled for the same event, the overview job always runs before review.

Autogen webhook:

- Runs on `pull_request` events when `autogen.enabled=true`, the action is in `autogen.events` (defaults to `opened`, `reopened`, `synchronize`), and at least one of `autogen.docs` or `autogen.tests` is true.

CLI review (GitHub PR by number):

```bash
go run ./cmd/autogit review --vcs-provider github --change-request 123
```

CLI review (local staged changes):

```bash
go run ./cmd/autogit review --vcs-provider github --head @staged
```

When `--head` is omitted in local workspace mode, it defaults to `@all` (staged + unstaged + untracked). Use `@staged` to limit to staged changes.

CLI review (publish review comments):

```bash
go run ./cmd/autogit review --vcs-provider github --change-request 123 --publish
```

CLI overview:

```bash
go run ./cmd/autogit overview --vcs-provider github --change-request 123
```

CLI replycomment (publish reply):

```bash
go run ./cmd/autogit replycomment --vcs-provider github --change-request 123 --comment-id 456789 --publish
```

CLI autogen (docs + tests, publish):

```bash
go run ./cmd/autogit autogen --vcs-provider github --change-request 123 --docs --tests --publish
```

See `go run ./cmd/autogit --help` for flags and defaults.

## Custom Recipe

- Repo-local configuration lives in `.autogit/config.toml`.
- Supported keys (high level): review ruleset, review suggestions/overview toggles, overview/reply/autogen extra guidance.
- All recipe paths are relative to `.autogit/` (for example `rules.md` is `.autogit/rules.md`).
- Example config: [Custom Recipe](/docs/custom-recipe.md).

## Troubleshooting

- `invalid signature` from `/webhook/github`:
  - `GITHUB_WEBHOOK_SECRET` does not match GitHub App webhook secret.
- `missing installation id` from `/webhook/github`:
  - webhook payload has no `installation.id` (app may not be installed correctly on repo/org).
- `failed to parse github app private key`:
  - verify PEM format in `GITHUB_APP_PRIVATE_KEY`.
- `failed to read github app private key file`:
  - verify `GITHUB_APP_PRIVATE_KEY` points to an existing readable PEM file.
