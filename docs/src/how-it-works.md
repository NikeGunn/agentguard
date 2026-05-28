# How it works

AgentGuard is a transparent stdio proxy.

```
┌─────────────────┐    spawn + pipe    ┌──────────────────┐    spawn + pipe    ┌────────────────┐
│  AI agent       │ ─────────────────▶ │  agentguard wrap │ ─────────────────▶ │  MCP server    │
│  (Claude Code,  │ ◀───────────────── │  (5-stage chain) │ ◀───────────────── │  (github, fs,  │
│   Cursor, …)    │                    │                  │                    │   slack, …)    │
└─────────────────┘                    └──────────────────┘                    └────────────────┘
```

`agentguard init` rewrites every MCP entry in each agent's config so
the agent spawns `agentguard wrap -- <original command>` instead of
the upstream server directly. From the agent's perspective nothing
changed; from the server's perspective nothing changed either.

In the middle, every JSON-RPC frame goes through five inspection
stages. The full pipeline must complete in under 5 ms p99 — that's
the design budget.

## The five stages

| # | Stage         | Cost   | Job                                                   |
|---|---------------|--------|-------------------------------------------------------|
| 0 | Transport     | <1 µs  | Per-session rate limit, frame-size cap                |
| 1 | Schema        | ~5 µs  | JSON-RPC 2.0 envelope validation                      |
| 2 | Attestation   | ~10 µs | Detect tool-schema drift between sessions (rug pull)  |
| 3 | Policy        | ~30 µs | YAML rule packs: allow/block/flag/redact              |
| 4 | Content scan  | ~20 µs | Regex secret redaction + injection signatures        |
| 5 | ML classify   | ~80 µs | Heuristic prompt-injection scorer with calibrated features |

A separate **circuit breaker** stage trips per-session after repeated
block/error verdicts and refuses traffic until the cooldown elapses.

Everything is recorded to a local SQLite database (WAL mode, 0600).
Nothing leaves your machine.
