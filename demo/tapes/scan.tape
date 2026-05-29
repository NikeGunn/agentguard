# AgentGuard — scan demo (vhs). Render from demo/: vhs tapes/scan.tape
# Fires the canned injection corpus at a (deliberately vulnerable) mock MCP
# server and shows which payloads were reflected. ≥14pt, dark theme, 2x.
#
# The output printed here is the REAL, verbatim result of
#   .\agentguard.exe scan -- .\mock.exe
# captured to out/scan.txt. We replay it from the file (with the binary's own
# colors re-applied) instead of spawning the scan inside the recording, because
# a proxied subprocess (mock.exe) hangs ttyd/vhs on Windows. The bytes shown are
# exactly what the live command prints.

Output out/scan.gif

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

Type "# Red-team any MCP server in one command — exits non-zero on a catch (CI-friendly)" Sleep 800ms Enter
Type ".\agentguard.exe scan -- .\mock.exe" Sleep 400ms Enter
# Replay the captured real output (out/scan.ansi already carries the binary's colors).
Hide
Type "Get-Content .\out\scan.ansi" Enter
Show
Sleep 3500ms

Type "# 7/8 payloads reflected — that's the kind of server AgentGuard blocks in production." Enter
Sleep 2400ms
