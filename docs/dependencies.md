# Dependencies

## Overview

The CLI provides `peer install` subcommands to install required dependencies.

## Behavior

- `peer install git` installs Git.
- `peer install gh --login` installs `gh` and prompts for `gh auth login` if needed.
- `peer install glab --login` installs `glab` and prompts for `glab auth login` if needed.
- `peer install opencode` installs OpenCode.
- `peer install quickstart` runs `gh --login` then `opencode`.
- `peer update self` updates the CLI to the latest stable release.
- `peer update skills --path <path>` updates installed peer skills at explicit paths (repeatable).
- If no `--path` is provided, `peer update skills` auto-discovers skills under project, user, and XDG config locations for `.agents/skills`, `.claude/skills`, `.codex/skills`, `.cursor/skills`, and `.windsurf/skills`.
- In non-interactive terminals, the CLI prints platform-specific instructions and exits with an error.

## OpenCode (opencode) Install Options

Official instructions: https://opencode.ai/docs

- Install script (macOS/Linux):
  - `curl -fsSL https://opencode.ai/install | bash`
- Homebrew:
  - `brew install anomalyco/tap/opencode`
- npm:
  - `npm install -g opencode-ai`
- Windows (Chocolatey):
  - `choco install opencode`
- Windows (Scoop):
  - `scoop install opencode`
- Windows (Mise):
  - `mise use -g github:anomalyco/opencode`
- Windows (Docker):
  - `docker run -it --rm ghcr.io/anomalyco/opencode`

## GitHub CLI (gh) Install Options

Official instructions: https://github.com/cli/cli/blob/trunk/docs

macOS:

- Homebrew: `brew install gh`
- Release binaries and installers: https://github.com/cli/cli/releases

Linux:

- Debian/Ubuntu (apt):
  - `(type -p wget >/dev/null || (sudo apt update && sudo apt install wget -y)) && sudo mkdir -p -m 755 /etc/apt/keyrings && out=$(mktemp) && wget -nv -O$out https://cli.github.com/packages/githubcli-archive-keyring.gpg && cat $out | sudo tee /etc/apt/keyrings/githubcli-archive-keyring.gpg > /dev/null && sudo chmod go+r /etc/apt/keyrings/githubcli-archive-keyring.gpg && sudo mkdir -p -m 755 /etc/apt/sources.list.d && echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" | sudo tee /etc/apt/sources.list.d/github-cli.list > /dev/null && sudo apt update && sudo apt install gh -y`
- Fedora/CentOS/RHEL (DNF5):
  - `sudo dnf install dnf5-plugins && sudo dnf config-manager addrepo --from-repofile=https://cli.github.com/packages/rpm/gh-cli.repo && sudo dnf install gh --repo gh-cli`
- Fedora/CentOS/RHEL (DNF4):
  - `sudo dnf install 'dnf-command(config-manager)' && sudo dnf config-manager --add-repo https://cli.github.com/packages/rpm/gh-cli.repo && sudo dnf install gh --repo gh-cli`
- Yum:
  - `type -p yum-config-manager >/dev/null || sudo yum install yum-utils && sudo yum-config-manager --add-repo https://cli.github.com/packages/rpm/gh-cli.repo && sudo yum install gh`
- Release binaries: https://github.com/cli/cli/releases

Windows:

- WinGet: `winget install --id GitHub.cli`
- MSI/EXE installers: https://github.com/cli/cli/releases

## GitLab CLI (glab) Install Options

Official instructions: https://gitlab.com/gitlab-org/cli/-/tree/main/docs/source

macOS:

- Homebrew: `brew install glab`

Linux:

- Debian/Ubuntu (WakeMeOps):
  - `curl -sSL https://raw.githubusercontent.com/upciti/wakemeops/main/assets/install_repository | sudo bash`
  - `sudo apt install glab`
- Fedora (DNF):
  - `sudo dnf install -y glab`

Windows:

- WinGet: `winget install glab.glab`
- Chocolatey: `choco install glab`
- Scoop: `scoop install glab`

## Git Install Options

Official instructions: https://git-scm.com/downloads

macOS:

- Homebrew: `brew install git`

Linux:

- Debian/Ubuntu (apt-get):
  - `sudo apt-get update && sudo apt-get install git -y`
- DNF:
  - `sudo dnf install -y git`
- Yum:
  - `sudo yum install -y git`

Windows:

- WinGet: `winget install --id Git.Git`
