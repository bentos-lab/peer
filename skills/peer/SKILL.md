---
name: peer
description: >
  CLI tool for working with code changes in Git repositories.

  Supports both:
  - local diffs (auto compare among refs, staged, unstaged, untracked)
  - pull requests or merge requests (PR/MR)

  Trigger this tool when the user asks to:
  - review code changes
  - create an overview of code changes
  - generate documentation or tests from code changes
  - reply to comments in a PR/MR
---

## Overview
* Skill version: dev

`peer` is a CLI that reviews and summarizes pull/merge requests, replies to PR comments, and can generate docs/tests/comments. It works both on PR/MR contexts and on local changes or diffs in a repository.

## Workflow

1. Choose the suitable action (review, overview, replycomment, autogen) and execution mode (PR/MR or local change/diff).
2. Must read the guidance in [reference guide](#reference-guide) of the corresponding action.
3. Run the appropriate subcommand follow by reference docs.
4. Interpret output and summarize results for the user. No need to inspect any more files. Note that only focus on [INFO] or above log message, ignore [DEBUG] or [TRACE].

## Reference Guide
[references/installation.md](references/installation.md): Installation steps.
[references/update.md](references/update.md): Self update and override skill guidance.
[references/dependencies.md](references/dependencies.md): Install dependencies guidance.
[references/review.md](references/review.md): Review a change request or local diff.
[references/overview.md](references/overview.md): Summarize a change request or local diff.
[references/replycomment.md](references/replycomment.md): Reply to a PR comment or a specific question.
[references/autogen.md](references/autogen.md): Generate docs/tests/comments for a change request or local diff.

## Note
- Before executing any command related to remote VCS, ask whether the result should be published (e.g., as PR/MR comments) or printed to the console. If this is a local command, no need to ask.
- Run `peer --help` to get help usage.
- The tool can run in very long time, up to 10 minutes, please keep patient when running these commands, even if no log printed, do not kill it.
- Load the smallest set of references that fits the task. Do not load every reference by default.
- Do not use this skill without repository access or required credentials, or for tasks outside supported subcommands.

## Default Operating Assumptions
- Do not assume unavailable credentials.
- Do not invent commands or flags not present in repository documentation.

## Execution Notes
- The tool is available as a global CLI: `peer`.
- Always use `-v` if the subcommand supports to log more values.

## Troubleshooting
1. If `gh` or `glab` has not authenticated yet or invalid token, follow guidance in [references/dependencies.md](references/dependencies.md):
* require user run authentication command for corresponding tool
* require read-permission of credential files.
* switch to higher permission mode (maybe the agent is running in sandbox mode, the tool has not read-permission to credential files).
2. If the CLI has not installed, install the tool via [installation.md](references/installation.md) and install dependencies via [dependencies](references/dependencies.md).
3. If the skill version does not match the CLI version, update the skill via [update.md](references/update.md).
4. If dependencies are mising, install them via [installation.md](references/installation.md).
