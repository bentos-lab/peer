# autogit

LLM-assisted pull/merge request reviewer with clean architecture and a simple CLI.

Docs: [Webhooks](/docs/webhooks.md) | [Configuration](/docs/configuration.md) | [Custom Recipe](/docs/custom-recipe.md) | [Architecture](/docs/architecture.md)

## Supported Flows

| Flow                               | Supported          |
| ---------------------------------- | ------------------ |
| GitHub webhook (`/webhook/github`) | :white_check_mark: |
| GitLab webhook (`/webhook/gitlab`) | :white_check_mark: |
| Local CLI review                   | :white_check_mark: |

## Install

### Stable

Linux/macOS
```bash
curl -fsSL https://raw.githubusercontent.com/sisu/autogit/main/install.sh | bash
```

Windows
```bash
iwr https://raw.githubusercontent.com/sisu/autogit/main/install.ps1 -useb | iex
```

### Latest

Prerequisites:
- Go `1.26`

```bash
go install ./cmd/autogit
```

## Quick Start

### Setup dependencies

Refer to [Dependencies Setup Guide](./docs/dependencies.md) for more details.

#### Required tools:
```bash
autogit install git
autogit install opencode
```

***Notes***
- Configure **Git** credentials to access private repositories.
- Authenticate with **Opencode** to use higher-performance coding models.


#### (Optional) Integration with remote VCS

- GitHub
```bash
autogit install gh --login
```

- GitLab
```bash
autogit install glab --login
```

### Webhook

1. Configure webhook environments and triggers. Refer to [Webhook setup guide](/docs/webhooks.md).

2. Run the webhook server:

```bash
autogit webhook --vcs-provider github
autogit webhook --vcs-provider gitlab
autogit webhook --vcs-provider github+gitlab
```

Webhook endpoints:

- `POST /webhook/github`
- `POST /webhook/gitlab`

### Local

Review change request 123 in the current repository and print the result to the console:
```bash
autogit review --change-request 123
```

If the repository uses a self-managed remote, specify the VCS provider and publish the result as a comment on the Pull Request:
```bash
autogit overview --vcs-provider github --change-request 123
```

Reply to a specific comment (issuecomment-1234567890) in change request 123 and print the result to the console:
```bash
autogit replycomment --vcs-provider github --change-request 123 --comment-id issuecomment-1234567890
```

Generate an overview for a specific repository and publish the result
```bash
autogit overview --repo https://github.com/user/repo.git --change-request 123 --publish
```

See `autogit --help` for full CLI usage.

## Custom Recipe

Repo-local configuration lives in `.autogit/config.toml`.

- Example config and keys: [Custom Recipe](/docs/custom-recipe.md).

## Environment

See [Configuration](/docs/configuration.md).
