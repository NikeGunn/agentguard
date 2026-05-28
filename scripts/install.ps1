# AgentGuard installer - Windows (PowerShell 5+).
#
# Usage:
#   iwr -useb https://agentguard.space/install.ps1 | iex
#
# This script is hardened against networks that block Invoke-WebRequest
# (some ISP DPI filters silently drop IWR while letting curl through).
# It tries IWR first, then transparently falls back to curl.exe (which
# ships with Windows 10 1803+).

[CmdletBinding()]
param(
    [string]$Version = "latest",
    [string]$InstallDir = "$env:USERPROFILE\.agentguard\bin",
    [string]$Repo = "NikeGunn/agentguard"
)

$ErrorActionPreference = "Stop"
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12 -bor [Net.SecurityProtocolType]::Tls13

function Download-Url {
    param([string]$Url, [string]$Out)
    # Try Invoke-WebRequest first.
    try {
        Invoke-WebRequest -Uri $Url -OutFile $Out -UseBasicParsing -TimeoutSec 30 -ErrorAction Stop
        return
    } catch {
        Write-Host "    (IWR failed: $($_.Exception.Message.Split([char]10)[0]) - retrying with curl.exe)" -ForegroundColor DarkYellow
    }
    # Fall back to curl.exe (works on networks that block IWR via SNI DPI).
    $curl = Get-Command curl.exe -ErrorAction SilentlyContinue
    if (-not $curl) { throw "Both Invoke-WebRequest and curl.exe failed. Network is blocking GitHub. Try a VPN." }
    & curl.exe -fsSL --tlsv1.2 -o $Out $Url
    if ($LASTEXITCODE -ne 0) { throw "curl.exe also failed (exit $LASTEXITCODE). Check network." }
}

function Fetch-JsonText {
    param([string]$Url)
    $tmpFile = [System.IO.Path]::GetTempFileName()
    try {
        Download-Url -Url $Url -Out $tmpFile
        return Get-Content $tmpFile -Raw
    } finally { Remove-Item $tmpFile -ErrorAction SilentlyContinue }
}

$arch = if ([Environment]::Is64BitOperatingSystem) {
    if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } else { "amd64" }
} else { throw "AgentGuard requires 64-bit Windows." }

if ($Version -eq "latest") {
    Write-Host "==> Resolving latest version..."
    $rel = Fetch-JsonText "https://api.github.com/repos/$Repo/releases/latest" | ConvertFrom-Json
    $Version = $rel.tag_name
}

$archive = "agentguard_$($Version.TrimStart('v'))_windows_$arch.zip"
$url = "https://github.com/$Repo/releases/download/$Version/$archive"
$checksumsUrl = "https://github.com/$Repo/releases/download/$Version/checksums.txt"

$tmp = New-Item -ItemType Directory -Path ([System.IO.Path]::Combine($env:TEMP, "agentguard-install-" + [System.Guid]::NewGuid()))
try {
    Write-Host "==> AgentGuard $Version  (windows/$arch)"
    Write-Host "==> Downloading $archive"
    Download-Url -Url $url          -Out (Join-Path $tmp $archive)
    Download-Url -Url $checksumsUrl -Out (Join-Path $tmp "checksums.txt")

    Write-Host "==> Verifying SHA-256"
    $expectedLine = Get-Content (Join-Path $tmp "checksums.txt") | Where-Object { $_ -match [Regex]::Escape(" $archive") }
    if (-not $expectedLine) { throw "no checksum for $archive in checksums.txt" }
    $expected = ($expectedLine -split '\s+')[0].ToLower()
    $actual = (Get-FileHash -Algorithm SHA256 (Join-Path $tmp $archive)).Hash.ToLower()
    if ($expected -ne $actual) { throw "SHA-256 mismatch! expected $expected, got $actual" }
    Write-Host "    OK  $actual" -ForegroundColor Green

    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
    Expand-Archive -Path (Join-Path $tmp $archive) -DestinationPath $tmp -Force
    $exe = Get-ChildItem -Path $tmp -Filter "agentguard.exe" -Recurse | Select-Object -First 1
    if (-not $exe) { throw "agentguard.exe not found inside $archive" }
    Move-Item -Path $exe.FullName -Destination (Join-Path $InstallDir "agentguard.exe") -Force

    Write-Host "==> Installed to $InstallDir\agentguard.exe"

    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if (-not ($userPath -split ';' -contains $InstallDir)) {
        [Environment]::SetEnvironmentVariable("Path", "$userPath;$InstallDir", "User")
        Write-Host "==> Added $InstallDir to user PATH (open a new shell for it to take effect)" -ForegroundColor Green
    }

    Write-Host ""
    Write-Host "Next steps:" -ForegroundColor Cyan
    Write-Host "  agentguard --version"
    Write-Host "  agentguard init --interactive"
    Write-Host "  agentguard dashboard"
}
finally {
    Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}
