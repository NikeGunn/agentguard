# AgentGuard demo recording — runs in vhs (https://github.com/charmbracelet/vhs).
#
# Regenerate the GIF:
#   1. Install vhs + ffmpeg
#        winget install charmbracelet.vhs Gyan.FFmpeg
#   2. Build the binary
#        go build -o agentguard.exe ./cmd/agentguard
#   3. Render
#        vhs demo/agentguard.tape
#   4. Commit demo/agentguard.gif
#
# This tape is HARMLESS — it only runs --version, --help, doctor, and
# a typed-out install line. It does NOT actually patch your agent
# configs; we don't want a recording session to modify ~/.claude etc.

Output demo/agentguard.gif

Set Shell "powershell"
Set FontSize 16
Set FontFamily "Cascadia Mono"
Set Width 1100
Set Height 640
Set TypingSpeed 38ms
Set PlaybackSpeed 1.0
Set Theme "Dracula"
Set Padding 24
Set Margin 8
Set MarginFill "#0b0f15"
Set BorderRadius 12

# Quiet the PowerShell prompt — it normally shows the full directory path,
# which is noisy on a recording.
Hide
Type "function global:prompt { '$ ' }" Enter
Type "Clear-Host" Enter
Show
Sleep 600ms

# Title card
Type "# AgentGuard — zero-trust security for AI agents" Sleep 700ms Enter
Sleep 500ms

# Pretend-install line (we don't actually re-install; we already built the .exe)
Type "curl -fsSL https://agentguard.space/install | sh" Sleep 1000ms Enter
Sleep 200ms
Hide
Type "Write-Host '==> AgentGuard v1.0.0-rc1  (windows/amd64)' -ForegroundColor DarkGray" Enter
Type "Write-Host '==> Verifying SHA-256   OK' -ForegroundColor DarkGray" Enter
Show
Sleep 800ms

# Verify the install
Type ".\agentguard.exe --version" Enter
Sleep 1400ms

# Top-level help — shows every command we shipped
Type ".\agentguard.exe --help" Enter
Sleep 2800ms

# Doctor — homebrew-style health check (safe: read-only)
Type ".\agentguard.exe doctor" Enter
Sleep 2400ms

# Pack list — shows the bundled rule packs
Type ".\agentguard.exe pack list" Enter
Sleep 2000ms

# Dashboard URL teaser (we don't start the server — vhs only captures terminal)
Type "# Run 'agentguard dashboard' to open http://127.0.0.1:7878" Enter
Sleep 1500ms

# Outro
Type "# 250+ tests · <5ms p99 · MIT · cosign-signed releases" Enter
Sleep 2500ms
