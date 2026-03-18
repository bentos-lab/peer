# bentos-backend

LLM-based pull/merge request reviewer built with Clean Architecture.

Current implementation is platform-agnostic at `usecase` level and supports:
- GitHub webhook flow
- GitLab webhook flow
- CLI flow

In this version, remote VCS behavior is comment-only (no merge blocking).

## Status

- `usecase` orchestration is implemented and tested.
- `core/v1` hardcoded review pack is implemented.
- OpenAI-compatible LLM adapter is implemented.
- GitHub/GitLab webhook handlers are implemented.
- GitHub/GitLab API clients are placeholders and still need real API integration.

## Project Structure

- `adapter`
  - `inbound`: webhook and CLI entry adapters
  - `outbound`: input providers, LLM adapter, publishers, VCS clients, rules
- `cmd`: entrypoints (`cmd/server/main.go`, `cmd/cli/main.go`)
- `config`: env-based runtime config
- `domain`: entities and value types
- `usecase`: platform-agnostic review orchestration
- `wiring`: dependency injection composition

## Quick Start

### Prerequisites

- Go `1.26`

### Install dependencies

```bash
go mod tidy
```

### Run tests

```bash
go test ./...
```

### Run server (webhooks)

```bash
go run ./cmd/server
```

Webhook routes:
- `POST /webhook/github`
- `POST /webhook/gitlab`

### Run CLI reviewer

```bash
go run ./cmd/cli
go run ./cmd/cli --all
go run ./cmd/cli --untracked
go run ./cmd/cli --all --untracked
go run ./cmd/cli --changed-files file1.go,file2.go
go run ./cmd/cli -a
go run ./cmd/cli -u
go run ./cmd/cli -a -u
go run ./cmd/cli -c file1.go,file2.go
go run ./cmd/cli --openai-base-url gemini
go run ./cmd/cli --openai-base-url openai --openai-model gpt-4.1-mini
go run ./cmd/cli --openai-base-url anthropic --openai-model claude-3-5-haiku-latest
go run ./cmd/cli --openai-base-url https://example.com/v1 --openai-model model-id --openai-api-key your-key
```

Notes:
- CLI argument parsing is handled in `cmd/cli/main.go`; inbound CLI adapter only receives parsed parameters.
- Review flags support shorthands: `-a` (`--all`), `-u` (`--untracked`), `-c` (`--changed-files`).
- `--openai-base-url` supports shortcuts: `gemini`, `openai`, `anthropic`.
- For shortcut URLs, if `--openai-model` is not provided, a shortcut default model is used.
- For full URLs, a model must be resolvable (`--openai-model` or `OPENAI_MODEL`).
- API key can be provided via `--openai-api-key` or `OPENAI_API_KEY`.

## Environment Variables

- `PORT` (default: `8080`)
- `OPENAI_BASE_URL` (default: `gemini`)
- `OPENAI_API_KEY` (required for real LLM calls)
- `OPENAI_MODEL` (default: `gemini-2.5-flash-lite`)

`.env` is loaded automatically when present in the current working directory.

## Behavior Summary

- Usecase does not know platform details.
- Inbound + wiring decide which concrete provider/publisher is injected.
- Review output:
  - file/area grouped messages (only where findings exist)
  - one summary message
- Active rule pack:
  - `core/v1` only (loaded from embedded template `usecase/rulepack/core_policy_v1.md`)

## Specs

- [Architecture Spec](/docs/architecture.md)
- [Review Spec](/docs/review-spec.md)
- [Agent Handoff](/docs/agent-handoff.md)
