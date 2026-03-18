# Repository Guidelines

- In chat replies, file references must be repo-root relative only (example: `extensions/bluebubbles/src/channel.ts:80`); never absolute paths or `~/...`.

## Project Structure & Module Organization

- This project follows Clean Architecture with clear separation of responsibilities.
    + `adapter`: Handles communication with external systems.
        + `inbound`: HTTP, CLI, RPC, WebSocket, Subcriber etc.
        + `outbound`: database, filesystem, cloud services, external APIs, cache, etc.
    + `cmd`: Application entrypoints. Bootstraps the app and starts servers/workers. No business logic allowed.
    + `build`: contains dockerfile for services.
    + `deploy`:
        + `local`: contains local deployment files (docker compose file).
        + `k8s`: contains k8s files.
    + `config`: Configuration definitions and loading logic. No business logic.
    + `domain`: Core business logic and domain models (entities, value objects, rules). Must not depend on outer layers.
    + `usecase`: Application logic. Orchestrates domain and defines interfaces for outbound dependencies.
    + `shared`: Reusable utilities and helpers not related to business logic.
    + `wiring`: Dependency injection and binding interfaces to concrete implementations. No business logic.

- Request flow:
  `cmd` (entrypoint) -> `inbound` adapter -> `usecase` -> `outbound` adapter.

- The domain layer is the core. All domain entities and logic can be used directly by both usecase and adapter layers.

# Documentation

- All documentation, design notes, and guidelines must be placed under the `docs` directory.
- Docs content must be generic: no personal device names/hostnames/paths; use placeholders like `user@gateway-host` and "gateway host".
- Exception: Keep `README.md`, `AGENTS.md` in root of this repo.
- Section cross-references: use anchors on root-relative paths (example: `[Hooks](/configuration#hooks)`).
- Always add new environments to `.env.example`.

## Prompt Authoring

- System prompts and task prompts must be clear, explicit, and aligned with repository rules.
- When a prompt needs detailed guidance, place that guidance in documentation under `docs/` and link to it from the prompt or AGENTS.
- Follow the prompt authoring guidelines in [System Prompt Guidelines](/docs/system-prompt-guidelines.md).

## Build, Test, and Development Commands

- Go version: 1.26
- Install dependencies using `go add <package_path>`
- Run all tests: `go test ./...`
- Run a specific test: `go test <package_or_test_path>`
- Run application entrypoint: `go run cmd/main.go --parameters`
- Commit message must contain a title and a detailed description of what changes, and the description must not exceed the maximum length.


## Coding Style and Naming Conventions

- Language: Go
- Test-first development.
- Follow standard Go conventions and idiomatic Go style.
- Adhere to SOLID principles.
- Add comments to all exported functions, global variables, and constants.
- Add brief inline comments for complex or non-obvious logic.
- Keep files under ~700 LOC as a guideline (not a strict limit). Refactor or split files when it improves clarity or testability.
- For enum, try using `type XXXEnum string.` Then define enum in the new type.
- Keep shared contracts/value types (for example enums used by both usecase and adapters) in `domain`; do not place them in `usecase` DTOs and then import them back into adapters.
- Before implementation, validate all guidelines and design decisions against Clean Architecture and SOLID principles.
- All function/method name must be neutral if is is posible.
- Refer to `./docs/system-prompt-guidelines.md` for building task (code agent) or system prompt (llm).

## Testing Guidelines

- Testing framework: `stretchr/testify`
- Test file naming must follow the `*_test.go` convention.
- Test file names should use the `<module_name>_test.go` pattern.
- Only test exported/public functions and methods; validate private logic through public APIs.
- Tests should match the structure and naming of the source files.
- Pure test additions or fixes generally do NOT require a changelog entry, unless they affect user-facing behavior or the user explicitly requests one.

## Planning new features

- Aggressive refactor code if it should be. Rename objects for consistent. Refer to [Refactor](#refactor-rules).

## Refactor rules

- Refer to [refactor.md](./docs/refactor.md).

## Release

* When the user asks to release a new version:
  * Refer to `RELEASE.md` for the full process
  * Prepare required changes, including:
    * Generating `CHANGELOG.md` based on commit history since the previous version
    * Updating version in `skills/peer/SKILL.md`
* Before performing any write or git operation (commit, tag, push):
  * Ask for user confirmation
* Do not execute the release automatically unless explicitly instructed by the user
