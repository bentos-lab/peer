# CLI Install Command

## Overview

The CLI provides `autogit install` subcommands to install required dependencies and guide authentication for GitHub CLI. Missing tools are installed on demand when invoked in interactive terminals.

## Behavior

- `autogit install gh --login` installs `gh` and prompts for `gh auth login` if needed.
- `autogit install opencode` installs OpenCode.
- `autogit install quickstart` runs `gh --login` then `opencode`.
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
