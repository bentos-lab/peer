$ErrorActionPreference = "Stop"

$Repo = "sisu/autogit"
$AppName = "autogit"

# Detect OS + ARCH
$OS = "windows"

switch ($env:PROCESSOR_ARCHITECTURE) {
    "AMD64" { $Arch = "amd64" }
    "ARM64" { $Arch = "arm64" }
    default {
        Write-Error "Unsupported architecture: $env:PROCESSOR_ARCHITECTURE"
        exit 1
    }
}

Write-Host "🔍 Detect: $OS/$Arch"

# Get latest version
$Release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest"
$Version = $Release.tag_name

Write-Host "📦 Latest version: $Version"

$FileName = "$AppName-$Version-$OS-$Arch.zip"
$Url = "https://github.com/$Repo/releases/download/$Version/$FileName"

$TempDir = New-Item -ItemType Directory -Path ([System.IO.Path]::GetTempPath()) -Name ("autogit_" + [System.Guid]::NewGuid())

$ZipPath = Join-Path $TempDir $FileName

Write-Host "⬇️ Downloading..."
Invoke-WebRequest -Uri $Url -OutFile $ZipPath

Write-Host "📂 Extracting..."
Expand-Archive -Path $ZipPath -DestinationPath $TempDir

# Install dir
$InstallDir = "$env:USERPROFILE\bin"

if (!(Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir | Out-Null
}

Move-Item "$TempDir\$AppName.exe" "$InstallDir\$AppName.exe" -Force

# Add to PATH if not exists
$UserPath = [Environment]::GetEnvironmentVariable("PATH", "User")

if ($UserPath -notlike "*$InstallDir*") {
    [Environment]::SetEnvironmentVariable("PATH", "$UserPath;$InstallDir", "User")
    Write-Host "➕ Added $InstallDir to PATH (restart terminal to apply)"
}

Remove-Item $TempDir -Recurse -Force

Write-Host ""
Write-Host "✅ Installed!"
Write-Host "👉 Run: $AppName --version"
