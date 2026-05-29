# AgentGuard — tail demo (vhs). Render from demo/: vhs tapes/tail.tape
# The viral TUI: a live, color-coded feed of every tool call. ≥14pt, dark, 2x.
#
# Pre-reqs (the wrapper run-all.ps1 does these — seeding the DB *outside* the
# tape, because spawning seed-demo inside ttyd hangs vhs on Windows):
#   go build -o agentguard.exe ./cmd/agentguard
#   .\agentguard.exe seed-demo --db .\out\tail-demo.db --count 120

Output out/tail.gif

Set Shell "powershell"
Set FontSize 15
Set FontFamily "Cascadia Mono"
Set Width 1280
Set Height 760
Set TypingSpeed 28ms
Set PlaybackSpeed 2.0
Set Theme "Dracula"
Set Padding 24
Set Margin 10
Set MarginFill "#0b0f15"
Set BorderRadius 12

Hide
Type "function global:prompt { '$ ' }" Enter
Type "Clear-Host" Enter
Show
Sleep 500ms

Type "# Live feed of every tool call across every agent — htop for AI agents" Sleep 700ms Enter
Type ".\agentguard.exe tail --once --db .\out\tail-demo.db --limit 18" Enter
Sleep 3500ms

Type "# + allow   ~ transform/flag   x block — color-coded, filterable, replayable" Enter
Sleep 2400ms
