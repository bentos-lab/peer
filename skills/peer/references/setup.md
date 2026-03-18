# peer install

Purpose: Install required CLI dependencies used by peer.

Subcommands:
- `peer install gh` install GitHub CLI (`gh`)
  - `--login` runs `gh auth login` after install
- `peer install glab` install GitLab CLI (`glab`)
  - `--login` runs `glab auth login` after install
- `peer install opencode` install OpenCode (`opencode`)
- `peer install git` install Git
- `peer install quickstart` install `gh` (with login) and `opencode`

Examples:
```bash
peer install git
peer install opencode
```
```bash
peer install gh --login
peer install glab --login
```
