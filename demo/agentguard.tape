# AgentGuard demo recording — runs in vhs (https://github.com/charmbracelet/vhs).
#
# To regenerate demo.gif:
#   1. Install vhs:  brew install vhs  (or scoop install vhs / winget install charmbracelet.vhs)
#   2. Build the binary in this repo:  go build -o agentguard ./cmd/agentguard
#   3. From the repo root:  vhs demo/agentguard.tape
#   4. The GIF lands at demo/agentguard.gif — commit it.
#
# vhs replays the script in a real terminal and renders deterministic frames,
# so the GIF is identical across machines.

Output demo/agentguard.gif

Set Shell "bash"
Set FontSize 16
Set FontFamily "JetBrainsMono Nerd Font Mono"
Set Width 1100
Set Height 640
Set TypingSpeed 35ms
Set PlaybackSpeed 1.0
Set Theme "Dracula"
Set Padding 24
Set Margin 8
Set MarginFill "#0b0f15"
Set BorderRadius 12

# 1 ─ Hero shot
Hide
Type "clear" Enter
Show
Sleep 700ms

# 2 ─ Install (we fake the install with a local symlink for the demo)
Type "# AgentGuard — zero-trust security for AI agents" Sleep 600ms Enter
Sleep 400ms
Type "curl -fsSL https://agentguard.dev/install | sh" Sleep 800ms Enter
Sleep 1200ms

# 3 ─ Verify
Type "agentguard --version" Enter
Sleep 1200ms

# 4 ─ Init — auto-detect every agent
Type "agentguard init --interactive" Enter
Sleep 2500ms
# Bubble Tea checklist — toggle, hit enter
Down Sleep 400ms
Down Sleep 400ms
Space Sleep 400ms
Up Sleep 400ms
Enter
Sleep 2500ms

# 5 ─ Doctor — homebrew-style health check
Type "agentguard doctor" Enter
Sleep 2200ms

# 6 ─ Tail the live stream
Type "agentguard tail" Sleep 400ms Enter
Sleep 3500ms
# Quit the TUI
Ctrl+C
Sleep 600ms

# 7 ─ Open the dashboard (just print the URL — recording the browser
#     is OBS territory, not vhs)
Type "agentguard dashboard --no-browser &" Enter
Sleep 1500ms
Type "# ⇒ http://127.0.0.1:7878" Enter
Sleep 1200ms

# 8 ─ Outro
Type "# 250+ tests · <5ms p99 · MIT · cosign-signed" Enter
Sleep 2000ms
