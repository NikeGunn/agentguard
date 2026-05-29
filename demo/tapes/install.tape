# AgentGuard — install demo (vhs). Render: vhs demo/tapes/install.tape
# Shows the one-line install + the agent-handoff story. HARMLESS: the install
# line is typed but the actual download is faked with echoes so a recording
# never mutates the machine. ≥14pt, dark theme, 2x playback per Phase 7 spec.

Output out/install.gif

Set Shell "powershell"
Set FontSize 16
Set FontFamily "Cascadia Mono"
Set Width 1200
Set Height 680
Set TypingSpeed 30ms
Set PlaybackSpeed 2.0
Set Theme "Dracula"
Set Padding 26
Set Margin 10
Set MarginFill "#0b0f15"
Set BorderRadius 12

Hide
Type "function global:prompt { '$ ' }" Enter
Type "Clear-Host" Enter
Show
Sleep 500ms

Type "# 1. Install — one line, OS auto-detected on agentguard.space" Sleep 600ms Enter
Type "iwr -useb https://agentguard.space/install.ps1 | iex" Sleep 800ms Enter
Hide
Type "Write-Host '==> AgentGuard v1.2.0  (windows/amd64)' -ForegroundColor DarkGray" Enter
Type "Write-Host '==> Verifying SHA-256    OK' -ForegroundColor Green" Enter
Type "Write-Host '==> Installed to ~\.agentguard\bin\agentguard.exe' -ForegroundColor DarkGray" Enter
Show
Sleep 900ms

Type ".\agentguard.exe --version" Enter
Sleep 1200ms

Type "# 2. Then just tell your AI agent: 'I installed AgentGuard, set it up.'" Sleep 700ms Enter
Type "#    It runs init, starts the dashboard, and proves it works — guided by" Sleep 200ms Enter
Type "#    the Skill AgentGuard ships into ~/.claude/skills/agentguard/." Sleep 1200ms Enter

Type "# Two pastes. Zero hand-edited configs. <5ms p99. MIT. cosign-signed." Enter
Sleep 2200ms
