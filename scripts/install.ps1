# AgentGuard installer — Windows (PowerShell 5+).
#
# Usage:
#   iwr -useb https://agentguard.dev/install.ps1 | iex
# Or pin to a version:
#   iwr -useb https://agentguard.dev/install.ps1 | iex; agentguardInstall -Version v1.0.0

[CmdletBinding()]
param(
    [string]$Version = "latest",
    [string]$InstallDir = "$env:USERPROFILE\.agentguard\bin",
    [string]$Repo = "agentguard/agentguard"
)

$ErrorActionPreference = "Stop"

$arch = if ([Environment]::Is64BitOperatingSystem) {
    if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } else { "amd64" }
} else { throw "AgentGuard requires 64-bit Windows." }

if ($Version -eq "latest") {
    Write-Host "==> Resolving latest version..."
    $rel = Invoke-RestMethod "https://api.github.com/repos/$Repo/releases/latest"
    $Version = $rel.tag_name
}

$archive = "agentguard_$($Version.TrimStart('v'))_windows_$arch.zip"
$url = "https://github.com/$Repo/releases/download/$Version/$archive"
$checksumsUrl = "https://github.com/$Repo/releases/download/$Version/checksums.txt"

$tmp = New-Item -ItemType Directory -Path ([System.IO.Path]::Combine($env:TEMP, "agentguard-install-" + [System.Guid]::NewGuid()))
try {
    Write-Host "==> AgentGuard $Version  (windows/$arch)"
    Write-Host "==> Downloading $archive"
    Invoke-WebRequest -Uri $url -OutFile (Join-Path $tmp $archive) -UseBasicParsing
    Invoke-WebRequest -Uri $checksumsUrl -OutFile (Join-Path $tmp "checksums.txt") -UseBasicParsing

    Write-Host "==> Verifying SHA-256"
    $expectedLine = Get-Content (Join-Path $tmp "checksums.txt") | Where-Object { $_ -match [Regex]::Escape(" $archive") }
    if (-not $expectedLine) { throw "no checksum for $archive in checksums.txt" }
    $expected = ($expectedLine -split '\s+')[0].ToLower()
    $actual = (Get-FileHash -Algorithm SHA256 (Join-Path $tmp $archive)).Hash.ToLower()
    if ($expected -ne $actual) { throw "SHA-256 mismatch! expected $expected, got $actual" }
    Write-Host "    OK  $actual"

    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
    Expand-Archive -Path (Join-Path $tmp $archive) -DestinationPath $tmp -Force
    $exe = Get-ChildItem -Path $tmp -Filter "agentguard.exe" -Recurse | Select-Object -First 1
    if (-not $exe) { throw "agentguard.exe not found inside $archive" }
    Move-Item -Path $exe.FullName -Destination (Join-Path $InstallDir "agentguard.exe") -Force

    Write-Host "==> Installed to $InstallDir\agentguard.exe"

    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if (-not ($userPath -split ';' -contains $InstallDir)) {
        [Environment]::SetEnvironmentVariable("Path", "$userPath;$InstallDir", "User")
        Write-Host "==> Added $InstallDir to user PATH (open a new shell for it to take effect)"
    }

    Write-Host ""
    Write-Host "Next: run 'agentguard init' to protect your installed AI agents."
}
finally {
    Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}
