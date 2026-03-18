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

## Build, Test, and Development Commands

- Go version: 1.26
- Install dependencies using `go add <package_path>`
- Run all tests: `go test ./...`
- Run a specific test: `go test <package_or_test_path>`
- Run application entrypoint: `go run cmd/main.go --parameters`


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


## Testing Guidelines

- Testing framework: `stretchr/testify`
- Test file naming must follow the `*_test.go` convention.
- Test file names should use the `<module_name>_test.go` pattern.
- Only test exported/public functions and methods; validate private logic through public APIs.
- Tests should match the structure and naming of the source files.
- Pure test additions or fixes generally do NOT require a changelog entry, unless they affect user-facing behavior or the user explicitly requests one.
