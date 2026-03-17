# autogit overview

Purpose: Generate a high-level summary for a change request.

Arguments:
- `--vcs-provider` provider name (use when auto-detection is not possible)
- `--repo` repository (URL or owner/repo)
- `--change-request` pull request number
- `--base` base ref
- `--head` head ref or `@staged`/`@all`
- `--publish` post overview result as pull request comments
- `--issue-alignment` enable issue alignment analysis for overview

Examples:
```bash
autogit overview --vcs-provider github --change-request 123
```
```bash
autogit overview --repo https://github.com/user/repo.git --change-request 123 --publish
```
