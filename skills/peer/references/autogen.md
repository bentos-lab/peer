# peer autogen

Purpose: Run autogen to generate tests/docs/comments for a change request.

Arguments:
- `--vcs-provider` provider name (use when auto-detection is not possible)
- `--repo` repository (URL or owner/repo)
- `--change-request` pull request number
- `--base` base ref
- `--head` head ref or `@staged`/`@all`
- `--publish` post autogen summary and push changes to PR branch
- `--docs` generate docs and code comments
- `--tests` generate tests

Example:
```bash
peer autogen --vcs-provider github --change-request 123 --docs --tests
```
