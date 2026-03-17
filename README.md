# autogit

LLM-assisted pull/merge request reviewer with clean architecture and a simple CLI.

Docs: [Webhooks](/docs/webhooks.md) | [Configuration](/docs/configuration.md) | [Custom Recipe](/docs/custom-recipe.md) | [Architecture](/docs/architecture.md)

## Supported Flows

| Flow                               | Supported          |
| ---------------------------------- | ------------------ |
| GitHub webhook (`/webhook/github`) | :white_check_mark: |
| GitLab webhook (`/webhook/gitlab`) | :white_check_mark: |
| Local CLI review                   | :white_check_mark: |

## Prerequisites

- Go `1.26`

## Install

```bash
go install ./cmd/autogit
```

## Quick start

### Install tools

Install and login required tools (GitHub or GitLab):

```bash
autogit install opencode
autogit install gh --login
autogit install glab --login
```

### Webhook

1. Configure webhook environments and triggers: [Webhook setup guide](/docs/webhooks.md).

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

Popular examples:

```bash
autogit review --vcs-provider github --change-request 123
autogit overview --vcs-provider gitlab --change-request 456
autogit replycomment --vcs-provider github --change-request 123 --comment-id 456789 --publish
```

See `autogit --help` for full CLI usage.

## Custom Recipe

Repo-local configuration lives in `.autogit/config.toml`.

- Example config and keys: [Custom Recipe](/docs/custom-recipe.md).

## Environment

See [Configuration](/docs/configuration.md).
