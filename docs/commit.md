# Commit Command

The `peer commit` command generates a conventional commit message using a coding agent, prints the message, and optionally commits changes in the current repository.

## Usage

```bash
peer commit
peer commit --staged
peer commit --confirm=true
peer commit --confirm=false
```

## Behavior

- When `--staged` is set, the command uses only staged changes to generate the message and commits the staged set.
- Without `--staged`, the command uses all local changes (staged + unstaged + untracked) to generate the message, stages everything, and commits.
- The generated commit message is printed before any commit.
- A confirmation prompt (`Commit with this message? [y/N]:`) is shown unless `--confirm` is explicitly set.
- Invalid `--confirm` values return an error.
- If no matching changes are found, the command returns an error and does not commit.

## Conventional Commit Format

The generator must output a conventional commit message with the following constraints:

- Format: `type(scope): summary` or `type: summary`
- Allowed types: `feat`, `fix`, `docs`, `refactor`, `test`, `chore`, `build`, `ci`, `perf`, `style`
- Summary length: 72 characters or fewer
- Optional body:
  - Up to 3 bullets
  - Each bullet 72 characters or fewer

## Flags

- `--staged`: commit staged changes only
- `--confirm`: set commit confirmation (`true`/`yes`=commit, `false`/`no`=print only)
- `--code-agent`: override coding agent name
- `--code-agent-provider`: override coding agent provider
- `--code-agent-model`: override coding agent model
- `--verbose`, `-v`: increase log verbosity
