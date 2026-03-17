# autogit install

Purpose: Install required CLI dependencies used by autogit.

Subcommands:
- `autogit install gh` install GitHub CLI (`gh`)
  - `--login` runs `gh auth login` after install
- `autogit install glab` install GitLab CLI (`glab`)
  - `--login` runs `glab auth login` after install
- `autogit install opencode` install OpenCode (`opencode`)
- `autogit install git` install Git
- `autogit install quickstart` install `gh` (with login) and `opencode`

Examples:
```bash
autogit install git
autogit install opencode
```
```bash
autogit install gh --login
autogit install glab --login
```
