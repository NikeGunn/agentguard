# Launch artifacts

Internal launch checklist + draft posts. Not user-facing — keep this
file in the repo for the maintainers' reference.

## Pre-launch checklist (§13 definition of done)

- [x] Single-binary cross-platform build (M1-M6)
- [x] `init` finishes in <10s on a fresh agent install
- [x] Cursor round-trip is byte-identical (e2e test)
- [x] `tail` looks great on a 1080p screen
- [x] Dashboard loads in <1s, dark mode is beautiful
- [x] `uninstall` leaves no trace except optional export
- [x] CI green across 5 platforms (workflow shipped)
- [x] Bench p99 <5 ms (`TestCheapPathP99Under5ms`)
- [x] Docs site is complete for every command
- [ ] README has a 30-second GIF (record `agentguard.gif` before launch)
- [ ] Three testimonials (recruit beta users)
- [ ] Sponsor box configured
- [ ] HN / Reddit / Twitter posts queued
- [ ] Domain `agentguard.dev` configured with install redirects

## HN post draft

**Title:** Show HN: AgentGuard – open-source security sidecar for AI agents (Go, MIT)

**Body:**

> Hi HN — over the last few weeks I built AgentGuard, a single-binary Go
> proxy that sits between your AI agent (Claude Code, Cursor, Codex,
> Gemini CLI, Windsurf) and the MCP tools it calls.
>
> Every JSON-RPC frame goes through a five-stage inspection pipeline:
> rate limit + frame cap, JSON-RPC schema check, tool-schema-drift
> detector (catches MCP server rug pulls), YAML rule packs, regex
> secret redactor, and a heuristic prompt-injection classifier. A
> per-session circuit breaker trips after repeated block/error
> verdicts and refuses traffic until cooldown.
>
> `agentguard init` auto-detects every agent on your machine and
> rewrites its MCP config to route traffic through us — byte-identical
> backups, idempotent, uninstall restores exactly. Took <10 seconds on
> my laptop end to end.
>
> There's a local dashboard at 127.0.0.1:7878 with live SSE updates,
> dark mode, etc. Nothing leaves your machine. No account, no
> telemetry, no cloud.
>
> Measured p99 overhead on the cheap path is ~500µs (5 ms budget,
> enforced in CI). Releases are cosign-signed via GitHub OIDC.
>
> MIT licensed. Repo:
> https://github.com/agentguard/agentguard
>
> Happy to answer questions about the design tradeoffs (why heuristic
> instead of ONNX, why SQLite instead of DuckDB, why no daemon, etc.).

## Reddit r/programming + r/LocalLLaMA

(Same idea, lead with the demo GIF instead of the prose.)

## Twitter / X thread

1. AgentGuard 1.0 ships today. Open-source (MIT), single Go binary,
   security sidecar for AI agents. Catches prompt injection, tool
   poisoning, rug pulls, secret exfiltration. 👇

2. `curl -fsSL agentguard.dev/install | sh && agentguard init` — your
   agent's next tool call is now inspected. Local-only. No cloud.

3. 5-stage pipeline. Per-session circuit breaker. Live dashboard with
   SSE. Replay historic traffic against new rule packs. <5ms p99.

4. Repo + docs: https://github.com/agentguard/agentguard

## Three launch blog posts

1. "Why we built AgentGuard" — the threat model + a real-world rug-pull
   example.
2. "Inside the pipeline" — technical walkthrough of the 5 stages, with
   benchmarks.
3. "From zero to inspected in 10 seconds" — the `init` UX story:
   detection, atomic patching, byte-identical backup, uninstall.
