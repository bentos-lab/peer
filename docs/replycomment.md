# Replycomment

The replycomment feature answers PR questions by running the coding agent against the PR diff and replying in the same GitHub thread.

## Webhook behavior
- Listens to `issue_comment` and `pull_request_review_comment` events with `action=created`.
- Requires a `@NAME` or `/NAME` trigger in the comment body, where `NAME` is set by `REPLYCOMMENT_TRIGGER_NAME` (default `autogitbot`).
- Ignores bot-authored comments to avoid loops.
- Fetches full thread history (PR conversation comments or review thread replies) for context.

## CLI usage
Use the `replycomment` subcommand:

```bash
autogit replycomment --repo owner/repo --change-request 123 --comment-id 456789 --comment
```

To answer a raw question without posting a comment:

```bash
autogit replycomment --repo owner/repo --change-request 123 --question "What changed in the service layer?"
```

## Safety and edit intent handling
Questions are sanitized by an LLM to remove edit instructions and convert edit requests into suggestions only. Unsafe or unsupported prompts return a refusal message.
