# autogit review

Purpose: Run a repository review for a change request.

Arguments:
- `--vcs-provider` provider name (use when auto-detection is not possible)
- `--repo` repository (URL or owner/repo)
- `--change-request` pull request number
- `--base` base ref
- `--head` head ref or `@staged`/`@all`
- `--publish` post review result as pull request comments
- `--suggest` enable suggested code changes in review findings

Example:
```bash
autogit review --change-request 123
```
