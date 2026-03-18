# Webhook Setup

This guide shows how to configure GitHub and GitLab webhooks, map values to environment variables, and run the webhook server.

## Where To Set Environment Variables

You can set variables either in a shell session or in a `.env` file in the working directory.

Shell example:

```bash
export GITHUB_APP_ID="123456"
export GITHUB_WEBHOOK_SECRET="your_webhook_secret"
export GITHUB_APP_PRIVATE_KEY="/path/to/github_app_private_key.pem"
```

`.env` example:

```bash
GITHUB_APP_ID=123456
GITHUB_WEBHOOK_SECRET=your_webhook_secret
GITHUB_APP_PRIVATE_KEY=/path/to/github_app_private_key.pem
```

## GitHub Webhook (GitHub App)

### Step 1: Create the GitHub App

1. Go to GitHub and create a new GitHub App for your organization or user.
2. Set the webhook URL to your server endpoint:

`https://<your-public-host>/webhook/github`

3. Set a webhook secret and save it.
4. Subscribe to the `Pull requests` event.
5. Set permissions:
1. `Contents`: `Read and write`
2. `Pull requests`: `Read and write`
3. `Issues`: `Read and write`
4. `Metadata`: `Read-only`

### Step 2: Generate the App private key

1. In the GitHub App settings, generate a new private key.
2. Download the `.pem` file and store it securely.

### Step 3: Install the bot

1. In your browser, install the GitHub App and add it to the target repository (or organization).

### Environment variable mapping

| Value | Environment variable | Where to get it |
| --- | --- | --- |
| App ID | `GITHUB_APP_ID` | GitHub App settings page, App ID field |
| Webhook secret | `GITHUB_WEBHOOK_SECRET` | The secret you set in the App webhook settings |
| App private key | `GITHUB_APP_PRIVATE_KEY` | Path to the downloaded `.pem` file or the PEM content |
| GitHub API base URL (optional) | `GITHUB_API_BASE_URL` | GitHub Enterprise API base URL, if not using `https://api.github.com` |

PEM inline example:

```bash
export GITHUB_APP_PRIVATE_KEY="-----BEGIN PRIVATE KEY-----\n...\n-----END PRIVATE KEY-----"
```

## GitLab Webhook (Personal Access Token)

### Step 1: Create a Personal Access Token (PAT)

1. In GitLab, create a PAT with these scopes:
1. `api`
2. `read_repository`
3. `write_repository`

### Step 2: Auto-register webhooks

Allow the server to auto-register hooks using the PAT.

### Step 3: Set webhook URL and secret

Webhook URL format:

`https://<your-public-host>/webhook/gitlab`

Webhook secret: choose a token and keep it consistent across projects.

### Environment variable mapping

| Value | Environment variable | Where to get it |
| --- | --- | --- |
| Personal access token | `GITLAB_TOKEN` | GitLab PAT you created |
| Webhook secret token | `GITLAB_WEBHOOK_SECRET` | The secret you set for GitLab webhooks |
| Public webhook URL | `GITLAB_WEBHOOK_URL` | The public URL pointing to `/webhook/gitlab` |
| GitLab API base URL (optional) | `GITLAB_API_BASE_URL` | Your GitLab API base URL, if not using `https://<gitlab-host>/api/v4` |
| GitLab host (optional) | `GITLAB_HOST` | Hostname used when `GITLAB_API_BASE_URL` is empty |
| Sync interval (optional) | `GITLAB_SYNC_INTERVAL_MINUTES` | Minutes between webhook syncs |
| Sync state path (optional) | `GITLAB_SYNC_STATE_PATH` | File path to persist webhook sync state |

Auto-sync requirements:
1. `GITLAB_WEBHOOK_URL` and `GITLAB_WEBHOOK_SECRET` must be set.
2. Invite the bot user to the target repository with Maintainer role.

## Trigger Workflow And Related Environment Settings

These environment variables control which webhook events trigger actions.

| Feature | Enable flag | Event/action settings | Defaults |
| --- | --- | --- | --- |
| Review | `REVIEW` | `REVIEW_EVENTS` | `REVIEW=true`, `REVIEW_EVENTS=opened,synchronize,reopened` |
| Overview | `OVERVIEW` | `OVERVIEW_EVENTS` | `OVERVIEW=true`, `OVERVIEW_EVENTS=opened` |
| Replycomment | `REPLYCOMMENT` | `REPLYCOMMENT_EVENTS`, `REPLYCOMMENT_ACTIONS`, `REPLYCOMMENT_TRIGGER_NAME` | `REPLYCOMMENT=true`, `REPLYCOMMENT_EVENTS=issue_comment,pull_request_review_comment`, `REPLYCOMMENT_ACTIONS=created`, `REPLYCOMMENT_TRIGGER_NAME=peerbot` |
| Autogen | `AUTOGEN` | `AUTOGEN_EVENTS`, `AUTOGEN_DOCS`, `AUTOGEN_TESTS` | `AUTOGEN=false`, `AUTOGEN_EVENTS=opened,reopened,synchronize`, `AUTOGEN_DOCS=false`, `AUTOGEN_TESTS=false` |

## Run The Webhook Server

GitHub:

```bash
peer webhook --vcs-provider github
```

GitLab:

```bash
peer webhook --vcs-provider gitlab
```

GitHub + GitLab:

```bash
peer webhook --vcs-provider github+gitlab
```

Webhook endpoints:

- `POST /webhook/github`
- `POST /webhook/gitlab`
