# peer installation and setup

Unix-based systems (Linux/macOS)
1. Install stable release:
```bash
curl -fsSL https://raw.githubusercontent.com/bentos-lab/peer/main/install.sh | bash
```
2. If the `peer` binary is not on your PATH, add the Go bin directory:
```bash
export PATH="$(go env GOPATH)/bin:$PATH"
```

Windows (PowerShell)
1. Install stable release:
```powershell
iwr https://raw.githubusercontent.com/bentos-lab/peer/main/install.ps1 -useb | iex
```
2. If the `peer` binary is not on your PATH, add the Go bin directory:
```powershell
$env:Path = "$(go env GOPATH)\bin;" + $env:Path
```

Dependencies and setup
- Required tools:
```bash
peer install git
peer install opencode
```
- Optional VCS integrations:
```bash
peer install gh --login
peer install glab --login
```
- Configure Git credentials for private repositories.
- Authenticate with Opencode for higher-performance coding models.
