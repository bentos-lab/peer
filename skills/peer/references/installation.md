# peer installation and setup

Purpose: Install the peer CLI and required dependencies.

## Install peer

Stable (Linux/macOS):
```bash
curl -fsSL https://raw.githubusercontent.com/bentos-lab/peer/master/install.sh | bash
```

Stable (Windows PowerShell):
```powershell
iwr https://raw.githubusercontent.com/bentos-lab/peer/master/install.ps1 -useb | iex
```

If the `peer` binary is not on your PATH, add the Go bin directory:
```bash
export PATH="$(go env GOPATH)/bin:$PATH"
```
