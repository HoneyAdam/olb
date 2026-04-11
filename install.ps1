# OpenLoadBalancer install script for Windows
# Usage: irm https://openloadbalancer.dev/install.ps1 | iex
#        or: irm https://raw.githubusercontent.com/openloadbalancer/olb/main/install.ps1 | iex
param()

$ErrorActionPreference = "Stop"

$Repo = "openloadbalancer/olb"
$Binary = "olb"
$InstallDir = if ($env:INSTALL_DIR) { $env:INSTALL_DIR } else { "$env:ProgramFiles\OpenLoadBalancer" }

# --- Detect architecture ---
$Arch = if ([Environment]::Is64BitOperatingSystem) { "amd64" } else { "amd64" }

# --- Determine version ---
$Tag = $env:VERSION
if (-not $Tag) {
    try {
        $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest" -ErrorAction Stop
        $Tag = $release.tag_name
    } catch {
        # Fallback: try to get tag from redirects
        Write-Host "[info] Could not query GitHub API. Set `$env:VERSION manually." -ForegroundColor Yellow
        exit 1
    }
}

Write-Host "[info] Installing OpenLoadBalancer $Tag for windows-$Arch" -ForegroundColor Cyan

# --- Download ---
$Filename = "${Binary}-windows-${Arch}.exe"
$Url = "https://github.com/$Repo/releases/download/$Tag/$Filename"

$TmpDir = [System.IO.Path]::GetTempPath()
$Target = Join-Path $TmpDir "$Binary.exe"

Write-Host "[info] Downloading $Url ..."
try {
    Invoke-WebRequest -Uri $Url -OutFile $Target -ErrorAction Stop
} catch {
    Write-Host "[error] Download failed: $_" -ForegroundColor Red
    exit 1
}

# --- Install ---
if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

$Dest = Join-Path $InstallDir "$Binary.exe"
Move-Item -Path $Target -Destination $Dest -Force

# --- Add to PATH if not present ---
$Path = [Environment]::GetEnvironmentVariable("Path", "Machine")
if ($Path -notlike "*$InstallDir*") {
    Write-Host "[info] Adding $InstallDir to system PATH..."
    [Environment]::SetEnvironmentVariable("Path", "$Path;$InstallDir", "Machine")
    $env:Path = "$env:Path;$InstallDir"
}

# --- Verify ---
$VersionOutput = & $Dest version 2>$null | Select-Object -First 1
Write-Host "[ok] Installed: $VersionOutput" -ForegroundColor Green

Write-Host ""
Write-Host "Run 'olb setup' to create an initial configuration, or:"
Write-Host "  olb start --config olb.yaml"
Write-Host ""
