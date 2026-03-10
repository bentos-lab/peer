# Autogen (Lazywork) Spec

## Objective

Automatically add missing tests, documentation, and code comments for the merge-base diff between `BASE` and `HEAD`.

## CLI Flow

The CLI subcommand is `autogen` and supports both local workspace and GitHub PR contexts.

Flags:
- `--docs`: generate docs and code comments.
- `--tests`: generate tests.
- `--publish`: when set with `--change-request`, post a PR summary and push changes to the PR head branch.
- `--base` / `--head`: diff anchors for local or remote execution (merge-base diff).
- `--repo`: optional repository URL or `owner/repo` slug (empty means current workspace).

`--docs` and `--tests` can be combined.

## Diff Scope

- Uses merge-base diff when both `BASE` and `HEAD` are available.
- When `HEAD` is `@staged` or `@all`, uses workspace diff commands.

## Publishing

- CLI mode prints added content blocks with file path and line ranges.
- GitHub publish mode:
  - posts a PR comment summarizing added tests/docs/comments,
  - includes the coding-agent report in the PR comment body,
  - commits and pushes changes to the PR head branch.

## Safety

- Coding agent must not commit or push.
- Git operations for publish mode are performed by the publisher, not by the agent.
