# Commit (local)

## Purpose
Generate a conventional commit message from local changes and optionally commit them.

## Usage

```bash
peer commit -vv
peer commit --staged -vv
peer commit --confirm=true -vv
peer commit --confirm=false -vv
```

## Behavior
- Without `--staged`, the command uses all local changes (staged + unstaged + untracked), stages everything, and commits.
- With `--staged`, only staged changes are used and committed.
- The generated commit message is printed before commit.
- The command prompts `Commit with this message? [y/N]:` unless `--confirm` is explicitly set.
- Invalid `--confirm` values return an error.
- If no matching changes are found, the command returns an error and does not commit.

## Common Flags
- `--staged`: commit staged changes only
- `--confirm`: set commit confirmation (`true`/`yes`=commit, `false`/`no`=print only)
- `--code-agent`: override coding agent name
- `--code-agent-provider`: override coding agent provider
- `--code-agent-model`: override coding agent model
- `--verbose`, `-v`: increase log verbosity
