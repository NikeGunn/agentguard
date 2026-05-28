# AgentGuard

> The zero-trust security layer that every AI agent forgot to install.
> One command. Works with Claude Code, Cursor, Codex, Gemini CLI, Windsurf, and any MCP/A2A client.

AgentGuard is an open-source, MIT-licensed, single-binary security sidecar for
AI agents. It transparently proxies MCP and A2A traffic and blocks prompt
injection, tool poisoning, rug-pulls, runaway loops, and credential
exfiltration — before the model or the network ever sees them.

## Install

```bash
# macOS / Linux
curl -fsSL https://agentguard.dev/install | sh

# Windows (PowerShell)
iwr -useb agentguard.dev/install.ps1 | iex

# npm (Cursor / Claude Code crowd)
npx @agentguard/cli init
```

## What you get

- Local-first. No account, no telemetry by default.
- Sub-5ms p99 proxy overhead on the cheap path.
- A real-time TUI (`agentguard tail`) and a beautiful dashboard at
  `http://localhost:7878`.
- Auto-discovery and auto-wiring for Claude Code, Cursor, Codex, Gemini CLI,
  and Windsurf.

## Status

Early — Milestone 1 (stdio proxy + SQLite store + `wrap` command) is the first
slice that landed. See `CHANGELOG.md` for what works today and
`docs/` for the full design.

## License

MIT. See [LICENSE](LICENSE).
