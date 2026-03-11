# Agent Handoff

## Repository Intent

This repository is a backend service for LLM-assisted PR/MR review with one shared, platform-agnostic usecase.

## What Is Stable

- Core contracts in `usecase/contracts.go`
- Usecase orchestration in `usecase/review_usecase.go`
- Message grouping in `usecase/message_builder.go`
- Single rule pack provider: `core/v1`
- Core policy template file: `usecase/rulepack/core_policy_v1.md` (embedded)
- OpenAI-compatible and coding-agent LLM adapters

## What Is Incomplete

- Real GitHub API integration:
  - changed file loading
  - comment posting
- Real GitLab API integration:
  - changed file loading
  - note posting
- Optional persistence/observability around review runs
- Production-grade webhook verification and auth hardening

## Next Recommended Tasks

1. Implement real GitHub VCS client in `adapter/outbound/vcs/github/client.go`.
2. Implement real GitLab VCS client in `adapter/outbound/vcs/gitlab/client.go`.
3. Add retry/backoff and structured logging around outbound calls.
4. Add contract tests for API client request/response mapping.
5. Add secure webhook signature/token verification.

## Testing Policy

- Keep test-first for `domain` and `usecase`.
- Add adapter tests when implementing real integrations.
- Run full suite with:

```bash
go test ./...
```

## Design Guardrails

- Keep `usecase` free of platform branches.
- Do not move shared types from `domain` into adapters.
- Keep new docs in `docs/` (except `README.md`, `AGENTS.md`).
- Keep docs generic and environment-neutral.
