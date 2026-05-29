# AgentGuard — agent-onboarding demo (vhs). Render: vhs tapes/onboarding.tape
# (run from the demo/ directory).
#
# Tells the story the landing page sells: the user pastes the "set it up for me"
# prompt into Claude Code, and the agent does the whole setup itself — verify,
# init (detect + patch configs), start the dashboard, and confirm a live verdict.
#
# SAFE: init runs against a throwaway --home (demo/out/fakehome) with a sample
# .claude.json, so the recording never touches the real ~/.claude.json. The
# narration "> " lines emulate Claude Code; the agentguard commands are real.
#
# Pre-reqs (the wrapper run-all.ps1 does these):
#   go build -o agentguard.exe ./cmd/agentguard     (with v1.2.0 ldflags)
#   a sample config at out/fakehome/.claude.json

Output out/onboarding.gif

Set Shell "powershell"
Set FontSize 15
Set FontFamily "Cascadia Mono"
Set Width 1320
Set Height 820
Set TypingSpeed 22ms
Set PlaybackSpeed 2.0
Set Theme "Dracula"
Set Padding 26
Set Margin 10
Set MarginFill "#0b0f15"
Set BorderRadius 12

Hide
Type "function global:prompt { '' }" Enter
Type "Clear-Host" Enter
Type "$HOME_FAKE = (Resolve-Path .\out\fakehome).Path" Enter
Show
Sleep 400ms

# The human pastes the landing-page prompt into Claude Code.
Type "# You: I just installed AgentGuard — set it up for me." Sleep 700ms Enter
Sleep 600ms

# Claude Code takes over. (Lines prefixed with the agent's '⏺' marker.)
Type "# ⏺ On it. First, confirming the binary works." Sleep 500ms Enter
Type ".\agentguard.exe --version" Enter
Sleep 1200ms
Type ".\agentguard.exe doctor" Enter
Sleep 2600ms

Type "# ⏺ Detecting your agents and routing their MCP servers through AgentGuard…" Sleep 500ms Enter
Type ".\agentguard.exe init --non-interactive --home $HOME_FAKE" Enter
Sleep 2800ms

Type "# ⏺ Done — your config is backed up at .claude.json.agentguard.bak." Sleep 600ms Enter
Type "# ⏺ Starting the local dashboard on http://127.0.0.1:7878 …" Sleep 700ms Enter
Sleep 500ms
Hide
Type "Write-Host '  AgentGuard dashboard listening on http://127.0.0.1:7878' -ForegroundColor Green" Enter
Show
Sleep 1200ms

Type "# ⏺ I sent one test tool call through the proxy. Here's the verdict:" Sleep 600ms Enter
Type ".\agentguard.exe tail --once --db .\out\dash.db --limit 6" Enter
Sleep 3200ms

Type "# ⏺ All set. Every MCP tool call you make is now inspected — prompt" Sleep 200ms Enter
Type "#   injection, exfiltration, rug-pulls and runaway loops get blocked" Sleep 200ms Enter
Type "#   before they reach the model. I won't disable it without asking." Sleep 1200ms Enter
Sleep 2400ms
