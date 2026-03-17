# autogit replycomment

Purpose: Reply to a specific PR comment with an LLM-generated response.

Arguments:
- `--vcs-provider` provider name (use when auto-detection is not possible)
- `--repo` repository (URL or owner/repo)
- `--change-request` pull request number
- `--comment-id` comment id to answer
- `--question` question text to answer
- `--publish` post reply as pull request comment (requires `--comment-id`)

Example:
```bash
autogit replycomment --vcs-provider github --change-request 123 --comment-id issuecomment-1234567890
```
