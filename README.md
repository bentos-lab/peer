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

Global:

- `LOG_LEVEL` (default: `info`)
- `OVERVIEW_ENABLED` (optional bool)
  - Server default when unset: `true`
  - CLI default when unset: `false`

OpenAI:

- `OPENAI_BASE_URL` (default: `gemini`)
- `OPENAI_API_KEY` (required for real LLM calls)
- `OPENAI_MODEL` (default: `gemini-2.5-flash-lite`)

Server (webhook-only):

- `PORT` (default: `8080`)

Server GitHub webhook flow:

- `GITHUB_WEBHOOK_SECRET` (required)
- `GITHUB_APP_ID` (required)
- `GITHUB_APP_PRIVATE_KEY` (required, PEM content or path to PEM file; escaped `\n` is supported for inline mode)
- `GITHUB_API_BASE_URL` (optional, default: `https://api.github.com`)

Examples:

- Inline PEM mode:
  - `GITHUB_APP_PRIVATE_KEY=-----BEGIN PRIVATE KEY-----\n...\n-----END PRIVATE KEY-----`
- File path mode:
  - `GITHUB_APP_PRIVATE_KEY=/run/secrets/github_app_private_key.pem`

`.env` is auto-loaded when present in current working directory.

## GitHub App Setup

1. Create a GitHub App.
2. Configure permissions:
- `Pull requests`: `Read and write`
- `Contents`: `Read-only`
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

## Run Server (Webhook Mode)

```bash
go run ./cmd/server
go run ./cmd/server --log-level warning
```

Webhook routes:

- `POST /webhook/github`

GitHub PR actions that trigger review:

- `opened`
- `synchronize`
- `reopened`

For each trigger, the service:

1. Loads changed files from GitHub.
2. Runs LLM review.
3. Posts inline review comments.
4. Posts one overview comment only for `opened`, and only when `OVERVIEW_ENABLED=true` (or unset, because server default is enabled).

## Run CLI Reviewer

```bash
go run ./cmd/cli --provider github
go run ./cmd/cli --provider github --repo org/repo
go run ./cmd/cli --provider github --repo https://github.com/org/repo.git
go run ./cmd/cli --provider github --repo git@github.com:org/repo.git
go run ./cmd/cli --provider github --head @staged
go run ./cmd/cli --provider github --head @all
go run ./cmd/cli --provider github --base main --head feature/ref
go run ./cmd/cli --provider github --change-request 123
go run ./cmd/cli --provider github --change-request 123 --comment
go run ./cmd/cli --overview
go run ./cmd/cli --suggest
go run ./cmd/cli --suggest=false
```

CLI notes:

- CLI uses authenticated GitHub CLI (`gh auth login`) for repo/PR resolution.
- `--provider` currently supports only `github`.
- `--repo` supports:
  - `owner/repo`
  - `https://github.com/owner/repo.git` (or `http://...`)
  - `git@github.com:owner/repo.git`
  - `ssh://git@github.com/owner/repo.git`
- `--change-request` and `--base`/`--head` are mutually exclusive.
- `--comment` requires `--change-request`.
- `--head` supports:
  - `@staged`: staged workspace changes (token mode, not a git ref).
  - `@all`: staged + unstaged + untracked workspace changes (token mode, not a git ref).
  - any git ref/commit (ref mode).
- If `--head` is empty, it defaults to:
  - `@staged` in local workspace mode (without `--repo`).
  - `HEAD` when `--repo` is provided.
- If `--base` is empty while `--head` is non-empty, `--base` defaults to `HEAD`.
- `@staged` and `@all` require local workspace mode (omit `--repo`).
- When `--repo` is provided, `--head` must be a real git ref/commit.
- GitHub App auth is for server webhook flow.
- `--overview` always generates overview and sends it to the mode's configured overview publisher/output.
- If `--overview` is not provided, CLI uses `OVERVIEW_ENABLED` when set; otherwise overview is disabled by default.
- Explicit CLI flag value (`--overview` or `--overview=false`) takes precedence over `OVERVIEW_ENABLED`.
- `--suggest` enables structured suggested code changes in findings.
- If `--suggest` is not provided, CLI uses `REVIEW_SUGGESTED_CHANGES_ENABLED`.
- Explicit CLI flag value (`--suggest` or `--suggest=false`) takes precedence over `REVIEW_SUGGESTED_CHANGES_ENABLED`.

## Troubleshooting

- `invalid signature` from `/webhook/github`:
  - `GITHUB_WEBHOOK_SECRET` does not match GitHub App webhook secret.
- `missing installation id` from `/webhook/github`:
  - webhook payload has no `installation.id` (app may not be installed correctly on repo/org).
- `failed to parse github app private key`:
  - verify PEM format in `GITHUB_APP_PRIVATE_KEY`.
- `failed to read github app private key file`:
  - verify `GITHUB_APP_PRIVATE_KEY` points to an existing readable PEM file.

## Specs

- [Architecture Spec](/docs/architecture.md)
- [Review Spec](/docs/review-spec.md)
- [Agent Handoff](/docs/agent-handoff.md)
