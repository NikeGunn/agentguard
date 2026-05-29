# AgentGuard -- robust vhs tape renderer for Windows.
#
# Why this exists: on Windows, vhs drives headless Chrome via the `rod` library
# and LEAKS that Chrome process on teardown -- vhs writes the .gif but then hangs
# instead of exiting. Left alone, every render leaks one headless Chrome; once a
# handful pile up (all sharing %TEMP%\rod\user-data) new renders hang at Chrome
# *startup*, before a single command runs. The symptom is a render that freezes
# right after the `Set ...` directives echo.
#
# So we do NOT wait for vhs to exit. We wait for the OUTPUT GIF to appear and
# stop growing, then kill vhs and reap its rod Chrome. We also reap stray rod
# Chromes before each render. The user's real Chrome (no %TEMP%\rod profile) is
# never touched.
#
# Usage (from demo/):
#   pwsh -File render-tapes.ps1 doctor scan tail install onboarding
#   pwsh -File render-tapes.ps1            # renders all tapes/*.tape

param([string[]]$Tapes)

$ErrorActionPreference = "Stop"
$pkgRoot = "$env:LOCALAPPDATA\Microsoft\WinGet\Packages"
function Find-Bin($glob, $exe) {
  (Get-ChildItem $pkgRoot -Recurse -Filter $exe -ErrorAction SilentlyContinue |
    Where-Object FullName -like "*$glob*" | Select-Object -First 1).Directory.FullName
}
$ff   = Find-Bin "Gyan.FFmpeg"        "ffmpeg.exe"
$vhs  = Find-Bin "charmbracelet.vhs"  "vhs.exe"
$ttyd = Find-Bin "tsl0922.ttyd"       "ttyd.exe"
if (-not ($ff -and $vhs -and $ttyd)) { throw "missing one of ffmpeg/vhs/ttyd -- winget install Gyan.FFmpeg charmbracelet.vhs tsl0922.ttyd" }
$env:PATH = "$ff;$vhs;$ttyd;$env:PATH"

$demo = $PSScriptRoot
Set-Location $demo

function Reap-RodChrome {
  Get-CimInstance Win32_Process -Filter "Name='chrome.exe'" -ErrorAction SilentlyContinue |
    Where-Object { $_.CommandLine -match 'Temp\\rod\\user-data' } |
    ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }
}

if (-not $Tapes -or $Tapes.Count -eq 0) {
  $Tapes = Get-ChildItem "$demo\tapes\*.tape" | ForEach-Object { $_.BaseName }
}

foreach ($name in $Tapes) {
  $tape = "$demo\tapes\$name.tape"
  if (-not (Test-Path $tape)) { Write-Host "skip $name (no tape)" -ForegroundColor DarkGray; continue }
  $out = "$demo\out\$name.gif"
  if (Test-Path $out) { Remove-Item $out -Force }

  Reap-RodChrome
  Get-Process vhs,ttyd -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue

  Write-Host "rendering $name …" -ForegroundColor Cyan -NoNewline
  $p = Start-Process vhs -ArgumentList "tapes/$name.tape" -PassThru -NoNewWindow `
        -RedirectStandardOutput "$demo\out\$name.rlog" -RedirectStandardError "$demo\out\$name.rerr"

  # Wait for the GIF to appear and stop growing (vhs writes it before it hangs).
  $deadline = (Get-Date).AddSeconds(90)
  $stableHits = 0; $lastSize = -1
  while ((Get-Date) -lt $deadline) {
    Start-Sleep -Milliseconds 800
    if (Test-Path $out) {
      $sz = (Get-Item $out).Length
      if ($sz -eq $lastSize -and $sz -gt 0) { $stableHits++ } else { $stableHits = 0 }
      $lastSize = $sz
      if ($stableHits -ge 3) { break }   # ~2.4s unchanged ⇒ fully written
    }
    if ($p.HasExited) { Start-Sleep -Milliseconds 400; break }
  }

  # Tear down vhs + its leaked rod Chrome regardless of whether vhs "exited".
  if (-not $p.HasExited) { $p.Kill() }
  Reap-RodChrome

  if ((Test-Path $out) -and (Get-Item $out).Length -gt 0) {
    Write-Host " ok ($([math]::Round((Get-Item $out).Length/1KB)) KB)" -ForegroundColor Green
  } else {
    Write-Host " FAILED -- see out/$name.rlog" -ForegroundColor Red
  }
}

Reap-RodChrome
Write-Host "done." -ForegroundColor Cyan
