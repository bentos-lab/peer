# peer

**Peer** is a CLI tool that analyzes your local changes or pull requests to generate concise overviews, review feedback, and actionable suggestions—helping you understand, document, and iterate on code faster.

## Supported Features

| Flow                         | Supported          |
| ---------------------------- | ------------------ |
| Review                       | :white_check_mark: |
| Generate overview            | :white_check_mark: |
| Auto-generate docs and tests | :white_check_mark: |
| Auto reply comment           | :white_check_mark: |
| Auto commit                  | :white_check_mark: |
| Per-repo custom recipe       | :white_check_mark: |
| Local CLI                    | :white_check_mark: |
| GitHub/Gitlab webhook        | :white_check_mark: |
| Integrated Agent Skill       | :white_check_mark: |

## Install

### Stable

Linux/macOS
```bash
curl -fsSL https://raw.githubusercontent.com/bentos-lab/peer/master/install.sh | bash
```

Windows
```bash
iwr https://raw.githubusercontent.com/bentos-lab/peer/master/install.ps1 -useb | iex
```

### Latest

Prerequisites:
- Go `1.26`

```bash
go install https://github.com/bentos-lab/peer/cmd/peer
```

### Auto-update

Auto update `peer` to the latest stable version:
```bash
peer update self
```

## Quick Start

### Setup dependencies

Refer to [Dependencies Setup Guide](./docs/dependencies.md) for more details.

#### Required tools:
```bash
peer install git
peer install opencode
```

Based on your remote VCS, install and authenticate with these following tools:
- GitHub
```bash
peer install gh --login
```

- GitLab
```bash
peer install glab --login
```

***Notes***
- Configure Git credentials to enable access to private repositories.
- Authenticate with **Opencode** to use higher-performance coding models.
- You can install above tools manually without using `peer install`. Refer to [Dependencies installation](docs/dependencies.md).

### Webhook

1. Configure webhook environments and triggers. Refer to [Webhook setup guide](/docs/webhooks.md).

2. Run the webhook server to listen and handle events:

```bash
peer webhook --vcs-provider github
peer webhook --vcs-provider gitlab
peer webhook --vcs-provider github+gitlab
```

### Local

- Review the local staged code changes in the current repository:
```bash
peer review --head @staged
```

- Review the local code changes (including staged and unstaged) in the current repository against `master`:
```bash
peer review --base master --head @all
```

- Generate overview of the change request #123 in the current repository:
```bash
peer overview --change-request 123
```

- If the repository uses a self-managed remote, explicitly specify the VCS provider:
```bash
peer overview --vcs-provider github --change-request 123
```

- Publish the overview as a comment on the change request:
```bash
peer overview --change-request 123 --publish
```

- Answer to a specific comment (#issuecomment-1234567890) in change request #123:
```bash
peer replycomment --change-request 123 --comment-id issuecomment-1234567890
```

- Publish the answer as a reply.
```bash
peer replycomment --change-request 123 --comment-id issuecomment-1234567890 --publish
```

- Generate a commit message and commit all local changes:
```bash
peer commit
```

- The commit flow uses local changes only and does not use change request metadata.

- Commit staged changes only:
```bash
peer commit --staged
```

*See `peer --help` for full CLI usage.*

## Custom Recipe

Repo-local configuration lives in `.peer/config.toml`.

- Example config and keys: [Custom Recipe](/docs/custom-recipe.md).

## Environment

See [Configuration](/docs/configuration.md).

## Skill

Use the following url to install `peer` skill into your coding agent

[https://github.com/bentos-lab/peer/tree/master/skills/peer](https://github.com/bentos-lab/peer/tree/master/skills/peer)

> ***Important***: In coding agents which support **sandbox** mode, you should run in **full permission** instead.
>
> Our CLI requires to read credentials from filesystem to interact with `git`, `gh`, `glab`, and `opencode`. Sandbox mode can prevent to use those tools correctly.

When you updated `peer`, you should also update the skill in your coding agent. Run this command to auto discover common skill directories and update them:
```bash
peer update skills
```

Or you can specify which skill directories should be updated:
```bash
peer update skills --path ~/.agent/skills/peer --path ./.agent/skills/peer
```
