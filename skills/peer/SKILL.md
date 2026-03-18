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
`peer` is a CLI that reviews and summarizes pull/merge requests, replies to PR comments, and can generate docs/tests/comments. It works both on PR/MR contexts and on local changes or diffs in a repository.

Load the smallest set of references that fits the task. Do not load every reference by default.

Do not use this skill without repository access or required credentials, or for tasks outside supported subcommands.

## Version

* Skill version: v0

* Before running any command:

  * Check CLI version using: `peer --version`
  * Extract the commit SHA from the version output.
  * If the Skill version does not match the CLI commit, update this skill from:
    https://github.com/bentos-lab/peer

### Update Procedure

1. Clone the repository to a temporary directory.
2. Checkout the commit SHA obtained from `peer --version`.
3. Verify that the path `peer/skills/peer/` exists and is valid.
4. Backup the current skill directory.
5. Replace the current skill contents with `peer/skills/peer/`.
6. Reload and re-read the updated skill files.

### Safety Rules

* Do not overwrite existing skill files without creating a backup.
* Abort the update if the target path is missing or invalid.
* Ensure the checked-out commit matches the CLI version exactly.

## Workflow
1. Install the CLI and dependencies.
2. Configure Git credentials and authenticate with Opencode (and optional VCS CLIs).
3. Choose execution mode (PR/MR or local change/diff).
4. Run the appropriate subcommand for the target diff.
5. Interpret output and summarize results for the user.

## Note
- When the user requests a review, ask whether they want an overview and/or suggested changes.
- When the user requests auto-generation, ask whether to generate documentation, tests, or both.
- Before executing any command, ask whether the result should be published (e.g., as PR/MR comments) or printed to the console.

## Reference Guide
[references/installation.md](references/installation.md): Installation steps and dependency setup.
[references/install.md](references/setup.md): Install dependency subcommands and login flags.
[references/review.md](references/review.md): Review a change request or local diff.
[references/overview.md](references/overview.md): Summarize a change request or local diff.
[references/replycomment.md](references/replycomment.md): Reply to a PR comment.
[references/autogen.md](references/autogen.md): Generate docs/tests/comments for a change request or local diff.

## Default Operating Assumptions
- Do not assume unavailable credentials.
- Do not invent commands or flags not present in repository documentation.
- Prefer safe, read-only operations unless explicitly requested.

## Execution Notes
- The tool is available as a global CLI: `peer`.
- Always run commands via terminal.
