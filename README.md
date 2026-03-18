# peer

**Peer** is a CLI tool that analyzes your local changes or pull requests to generate concise overviews, review feedback, and actionable suggestions—helping you understand, document, and iterate on code faster.


Docs: [Webhooks](/docs/webhooks.md) | [Configuration](/docs/configuration.md) | [Custom Recipe](/docs/custom-recipe.md) | [Architecture](/docs/architecture.md)

## Supported Flows

| Flow                               | Supported          |
| ---------------------------------- | ------------------ |
| GitHub webhook (`/webhook/github`) | :white_check_mark: |
| GitLab webhook (`/webhook/gitlab`) | :white_check_mark: |
| Local CLI review                   | :white_check_mark: |
| Integrated Agent Skill             | :white_check_mark: |

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

## Auto-update

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

***Notes***
- Configure **Git** credentials to access private repositories.
- Authenticate with **Opencode** to use higher-performance coding models.


#### (Optional) Integration with remote VCS

- GitHub
```bash
peer install gh --login
```

- GitLab
```bash
peer install glab --login
```

### Webhook

1. Configure webhook environments and triggers. Refer to [Webhook setup guide](/docs/webhooks.md).

2. Run the webhook server:

```bash
peer webhook --vcs-provider github
peer webhook --vcs-provider gitlab
peer webhook --vcs-provider github+gitlab
```

Webhook endpoints:

- `POST /webhook/github`
- `POST /webhook/gitlab`

### Local

Review change request 123 in the current repository and print the result to the console:
```bash
peer review --change-request 123
```

If the repository uses a self-managed remote, specify the VCS provider and publish the result as a comment on the Pull Request:
```bash
peer overview --vcs-provider github --change-request 123
```

Reply to a specific comment (issuecomment-1234567890) in change request 123 and print the result to the console:
```bash
peer replycomment --vcs-provider github --change-request 123 --comment-id issuecomment-1234567890
```

Generate an overview for a specific repository and publish the result
```bash
peer overview --repo https://github.com/user/repo.git --change-request 123 --publish
```

See `peer --help` for full CLI usage.

## Custom Recipe

Repo-local configuration lives in `.peer/config.toml`.

- Example config and keys: [Custom Recipe](/docs/custom-recipe.md).

## Environment

See [Configuration](/docs/configuration.md).

## Skill

Use the following url to install `peer` skill into your coding agent

```
https://github.com/bentos-lab/peer/tree/master/skills/peer
```

> ***Important***: In coding agents which support **sandbox** mode, you should run with **full permission** instead.
>
> Our CLI requires to read credentials from filesystem to interact with git, github, gitlab, opencode.

When you updated the tool, you should also update the skill in your coding agent, run this command to auto-discover common skill patterns:
```bash
peer update skills
```

Or specify which skill folder should be updated:
```bash
peer update skills --path ~/.agent/skills/peer
```
