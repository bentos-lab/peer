# peer install

Purpose: Install required CLI dependencies used by peer.

## Subcommands and flags

- `peer install gh`
  - Flags: `--login` (default: `false`) runs `gh auth login` after install
  - Allowed values: `--login` is a boolean flag
- `peer install glab`
  - Flags: `--login` (default: `false`) runs `glab auth login` after install
  - Allowed values: `--login` is a boolean flag
- `peer install opencode`
  - Flags: none
- `peer install git`
  - Flags: none
- `peer install quickstart`
  - Flags: none
  - Behavior: installs `gh` with login and `opencode`

## Examples

Install required tools:
```bash
peer install git
peer install opencode
```

Install GitHub CLI and log in:
```bash
peer install gh --login
```

Install GitLab CLI and log in:
```bash
peer install glab --login
```

Quickstart:
```bash
peer install quickstart
```
