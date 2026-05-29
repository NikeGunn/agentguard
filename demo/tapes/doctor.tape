# AgentGuard — doctor demo (vhs). Render: vhs demo/tapes/doctor.tape
# Read-only health check; safe to record. ≥14pt, dark theme, 2x.

Output out/doctor.gif

Set Shell "powershell"
Set FontSize 16
Set FontFamily "Cascadia Mono"
Set Width 1200
Set Height 720
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

Type "# Homebrew-style health check — every finding is ✓ / ⚠ / ✗ with a hint" Sleep 700ms Enter
Type ".\agentguard.exe doctor" Enter
Sleep 3000ms

Type "# Exit 0 = ship-ready. Run it in CI as a gate." Enter
Sleep 2000ms
