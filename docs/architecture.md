# Architecture Spec

## Purpose

Define the architecture and dependency rules for this repository so implementation stays platform-agnostic in `usecase`.

## Layer Boundaries

- `domain`
  - Contains entities, value objects, enums, and shared types.
  - Must not depend on `usecase`, `adapter`, `wiring`, `cmd`, or `config`.
- `usecase`
  - Contains orchestration logic and interface contracts (ports).
  - Depends on `domain` only.
  - Must not branch by platform (`github`, `gitlab`, `cli`).
  - Shared application contracts for generic LLM generation live in `usecase/contracts`.
- `adapter/inbound`
  - Parses input from HTTP/CLI and maps to `usecase.ReviewRequest`.
  - No business logic.
- `adapter/outbound`
  - Concrete implementations for ports:
    - loading changed content
    - calling LLM
    - publishing review output
- `wiring`
  - Builds object graph and injects concrete adapters into usecase.
- `cmd`
  - Entrypoints only.

## Required Flow

`cmd` -> `adapter/inbound` -> `usecase` -> `adapter/outbound`

## Platform-Agnostic Constraint

`usecase` can only rely on ports:
- `ReviewInputProvider`
- `RulePackProvider`
- `LLMReviewer`
- `ReviewResultPublisher`

`usecase` must not inspect platform-specific fields or event types.

## Current Implementations

- Inbound
  - GitHub: `adapter/inbound/http/github`
  - GitLab: `adapter/inbound/http/gitlab`
  - CLI: `adapter/inbound/cli`
- Outbound
  - Input providers:
    - `adapter/outbound/input/github`
    - `adapter/outbound/input/gitlab`
    - `adapter/outbound/input/cli`
  - LLM:
    - Generic provider client: `adapter/outbound/llm/openai`
    - Review translator adapter: `adapter/outbound/reviewer/llm`
  - Publishers:
    - `adapter/outbound/publisher/github`
    - `adapter/outbound/publisher/gitlab`
    - `adapter/outbound/publisher/cli`
  - Rules:
    - `usecase/rulepack/core_provider.go`

## Non-Goals in Current Version

- Merge-blocking checks/statuses.
- Static analysis engine like SonarQube.
- Fixed Clean Architecture/SOLID enforcement rules inside review prompts.
