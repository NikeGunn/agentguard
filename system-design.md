# AgentGuard — System Design

> **Tagline:** The zero-trust security layer that every AI agent forgot to install.
> One command. Works with Claude Code, Cursor, Codex, Gemini CLI, Windsurf, and any MCP/A2A client.

---

## 1. Mission

AI agents in 2026 talk to the world through two protocols: **MCP** (Model Context Protocol — agent-to-tool) and **A2A** (Agent-to-Agent). Both have exploded in adoption and both are insecure by default:

- 52% of public MCP servers are abandoned (no patches, no maintainers)
- 8,000+ MCP servers in registries ship with zero authentication
- Tool poisoning, indirect prompt injection, and rug-pull attacks have no production-grade defense in the field
- Infinite agent loops have burned five-figure cloud bills (Braintrust documented $47k in 11 days)
- Multi-tenant isolation for the Claude Agent SDK is DIY

**AgentGuard** is a local-first, zero-config security sidecar that sits between an agent and the world. It intercepts every MCP and A2A call, runs a defense-in-depth pipeline against it, and only lets the safe ones through. It also kills runaway loops, caps spend per tool, and gives developers a real-time dashboard of what their agents are actually doing.

It is **MIT-licensed**, **a single Go binary** (≈18 MB), and the entire install is one shell command.

---

## 2. Design Principles

| # | Principle | Why it matters |
|---|-----------|----------------|
| 1 | **10-second install, zero-config default** | If setup takes more than one command, no one ships it. Defaults must be secure-out-of-the-box. |
| 2 | **Sub-5ms proxy overhead at p99** | Agents make hundreds of tool calls per session. Latency is the first complaint. |
| 3 | **Local-first, cloud-optional** | The data is the developer's. Cloud sync is opt-in. Free tier needs no account. |
| 4 | **Protocol-pure** | Don't fork MCP or A2A. Be a transparent proxy. The day Anthropic ships native defenses, AgentGuard composes, doesn't conflict. |
| 5 | **Read-only by default, write-on-approve** | Tool calls that mutate state require explicit allowlist or human-in-the-loop confirmation. |
| 6 | **Observable everything** | Every decision (allowed, blocked, flagged) is a structured event with a reason code. No black boxes. |
| 7 | **Composable with what exists** | If a user already runs Langfuse or Braintrust, AgentGuard ships traces to them via OpenTelemetry. We are not an observability competitor. |
| 8 | **Fail open in dev, fail closed in prod** | Mode is explicit. A dev should never get blocked debugging at 2am. A production agent should never leak data. |

---

## 3. Architecture at a Glance

```
                                                            ┌──────────────────────────┐
                                                            │ User's MCP / A2A Servers │
                                                            │  (filesystem, GitHub,    │
                                                            │   Slack, Notion, etc.)   │
                                                            └────────────▲─────────────┘
                                                                         │
                                                              ALLOW / BLOCK / TRANSFORM
                                                                         │
┌─────────────────────────┐    stdio / streamable-HTTP    ┌──────────────┴─────────────┐
│  Claude Code / Cursor / │ ◄═══════════════════════════► │     AgentGuard Sidecar     │
│  Codex / Gemini CLI /   │    (transparent passthrough)  │  (single Go binary, local) │
│  Windsurf / Custom SDK  │                                │                            │
└─────────────────────────┘                                │  ┌──────────────────────┐  │
                                                            │  │  Inspection Pipeline │  │
                                                            │  │  (see Section 5)     │  │
                                                            │  └──────────────────────┘  │
                                                            │  ┌──────────────────────┐  │
                                                            │  │  Local SQLite store  │  │
                                                            │  │  + DuckDB analytics  │  │
                                                            │  └──────────────────────┘  │
                                                            │  ┌──────────────────────┐  │
                                                            │  │  Dashboard (Bun + )  │  │
                                                            │  │  SvelteKit on :7878  │  │
                                                            │  └──────────────────────┘  │
                                                            └────────────────────────────┘
                                                                         │
                                                                         │ (optional)
                                                                         ▼
                                                            ┌────────────────────────────┐
                                                            │  AgentGuard Cloud (Pro)    │
                                                            │  - team policy sync        │
                                                            │  - centralized audit log   │
                                                            │  - SSO, RBAC, alerting     │
                                                            └────────────────────────────┘
```

The agent thinks it's talking directly to its MCP servers. AgentGuard is invisible until it needs to act. When it does, it produces an event the developer can see, replay, and override.

---

## 4. Tech Stack (2026-current choices, with reasoning)

The stack is picked for: (a) single-binary distribution, (b) speed, (c) ecosystem maturity, (d) what 2026 developers actually want to read.

### 4.1 Core sidecar — **Go 1.23+**
- Single static binary (no runtime to install)
- Goroutines per connection scale to thousands of concurrent tool calls with no thread pool tuning
- Mature stdlib for net/http, JSON-RPC, TLS, OS signals
- Cross-compile to macOS (arm64/amd64), Linux (arm64/amd64), Windows (amd64) from one CI job
- You (Nikhil) already write Go for PalikaBook — zero learning curve

### 4.2 Embedded datastore — **SQLite (via modernc.org/sqlite, pure-Go)**
- No CGO → cross-compilation stays trivial
- Local-first, file-based, zero ops
- WAL mode for concurrent reads while the sidecar writes events
- We use SQLite for OLTP (events, policies, sessions) and **DuckDB** in-process for the dashboard's analytical queries (groupings, time-series rollups)

### 4.3 Embedded ML — **ONNX Runtime (Go bindings)**
- We ship a small (~80 MB) fine-tuned prompt-injection classifier (DeBERTa-v3-small or a distilled equivalent) as an ONNX model
- Inference on CPU is sub-50ms — fine because we only call it on suspicious payloads after cheap regex/heuristic filters fail
- ONNX is the cross-runtime standard in 2026; the same model works on Apple Silicon, x86, and ARM Linux

### 4.4 Dashboard — **SvelteKit + Bun + Tailwind v4**
- Bun runtime: dashboard ships as a single bundled JS file the Go binary serves via embed.FS
- SvelteKit because it's the smallest-bundle, fastest-rendering modern framework and Claude Code knows it well
- Tailwind v4 (CSS-first config, no PostCSS) keeps the styling layer tiny
- Real-time UI via Server-Sent Events from the Go core (no separate WebSocket server)

### 4.5 Telemetry export — **OpenTelemetry (OTLP/HTTP)**
- Every event becomes a span with structured attributes
- Default exporter is local (SQLite), but users can point at Langfuse, Braintrust, Datadog, Honeycomb, Jaeger, or any OTLP-compatible backend with one env var

### 4.6 CLI — **Cobra + Bubble Tea**
- Cobra for command structure (`agentguard init`, `agentguard scan`, `agentguard logs`)
- Bubble Tea for the gorgeous interactive setup wizard and the `tail` command (think `lazygit` for agent traffic)
- Charm.sh's libraries are the gold standard for terminal UX in 2026

### 4.7 Install layer — **shell installer + Homebrew + Scoop + npm wrapper**
- Primary: `curl -fsSL agentguard.dev/install | sh` (detects OS+arch, downloads signed binary, verifies, drops into `~/.agentguard/bin`)
- macOS: `brew install agentguard`
- Windows: `scoop install agentguard`
- For npm-native devs (Cursor/Claude Code users): `npx @agentguard/cli init` wraps the same installer so it feels native to their muscle memory

### 4.8 Cloud (Pro tier, optional) — **Cloudflare Workers + D1 + R2**
- Same edge-first stack you know from ScanVault
- Workers for the policy-sync and audit-log ingest API
- D1 for relational data (orgs, users, policies)
- R2 for blob storage (rule packs, encrypted audit-log batches)
- Stripe for billing
- Clerk for auth (SSO without writing it)

---

## 5. The Inspection Pipeline (the heart of the system)

Every incoming MCP/A2A message goes through a deterministic pipeline. Each stage either passes, blocks, or transforms the message. Stages are ordered cheapest-first so most traffic never hits the expensive stages.

```
                ┌─────────────────────────────────────────────┐
   incoming ──► │ Stage 0: Transport guard                    │
   message      │   - TLS, auth header check, rate limit      │
                └────────────────────┬────────────────────────┘
                                     ▼
                ┌─────────────────────────────────────────────┐
                │ Stage 1: Schema validator                   │
                │   - JSON-RPC well-formedness                │
                │   - MCP/A2A spec conformance                │
                │   - reject malformed early                  │
                └────────────────────┬────────────────────────┘
                                     ▼
                ┌─────────────────────────────────────────────┐
                │ Stage 2: Server attestation                 │
                │   - is this server in the registry?         │
                │   - has the tool schema changed since last  │
                │     approval? (rug-pull detection)          │
                │   - is the upstream binary signature valid? │
                └────────────────────┬────────────────────────┘
                                     ▼
                ┌─────────────────────────────────────────────┐
                │ Stage 3: Policy engine (Rego / OPA-style)   │
                │   - allowlists, denylists                   │
                │   - per-tool budgets, per-server quotas     │
                │   - sensitive-tool requires-approval rules  │
                └────────────────────┬────────────────────────┘
                                     ▼
                ┌─────────────────────────────────────────────┐
                │ Stage 4: Content scanner (cheap)            │
                │   - regex bank for known injection patterns │
                │   - secret detection (AWS keys, tokens)     │
                │   - PII detection (emails, SSNs by locale)  │
                └────────────────────┬────────────────────────┘
                                     ▼
                ┌─────────────────────────────────────────────┐
                │ Stage 5: ML classifier (expensive)          │
                │   - only invoked if Stage 4 finds anomalies │
                │     or the tool returns >N tokens           │
                │   - ONNX prompt-injection model             │
                │   - returns confidence score                │
                └────────────────────┬────────────────────────┘
                                     ▼
                ┌─────────────────────────────────────────────┐
                │ Stage 6: Loop / cost circuit breaker        │
                │   - detect repeated identical tool calls    │
                │   - enforce session-level token & USD caps  │
                └────────────────────┬────────────────────────┘
                                     ▼
                ┌─────────────────────────────────────────────┐
                │ Stage 7: Audit & forward                    │
                │   - structured event written to SQLite      │
                │   - OTLP span emitted                       │
                │   - message forwarded to upstream / client  │
                └─────────────────────────────────────────────┘
```

Critically, the pipeline runs in **both directions**:
- **Outbound** (agent → tool): we validate the agent isn't being tricked into a destructive call.
- **Inbound** (tool → agent): we scan tool *results* for indirect prompt injection (the GitHub MCP heist pattern — malicious content inside a legitimate response).

---

## 6. The 10-Second Install Story

The install must be one command. Here's exactly what happens.

### 6.1 macOS / Linux
```bash
curl -fsSL https://agentguard.dev/install | sh
```

What runs:
1. Detect OS + arch
2. Download signed binary from GitHub Releases (≈18 MB)
3. Verify SHA-256 and Cosign signature
4. Drop into `~/.agentguard/bin/agentguard`
5. Add to PATH (idempotent — checks `.zshrc`, `.bashrc`, `.config/fish/config.fish`)
6. Run `agentguard init` automatically with sensible defaults
7. Detect installed agents (Claude Code, Cursor, Codex, Gemini CLI) by their config-file fingerprints
8. **Auto-patch each detected agent's MCP config** to route through AgentGuard
9. Print the dashboard URL: `http://localhost:7878`

### 6.2 Windows
```powershell
iwr -useb agentguard.dev/install.ps1 | iex
```

Same flow, PowerShell version. Or `scoop install agentguard`.

### 6.3 npm one-liner (for the Node-native crowd)
```bash
npx @agentguard/cli init
```

Wraps the shell installer, identical result. This is the line most Cursor/Claude Code users will run because it matches their muscle memory.

### 6.4 What "auto-patch" means

For Claude Code, AgentGuard finds the user's MCP config at `~/.claude/mcp_servers.json` (or `.claude.json` depending on version) and rewrites entries like:

```jsonc
// BEFORE
{
  "github": {
    "command": "npx",
    "args": ["-y", "@modelcontextprotocol/server-github"]
  }
}

// AFTER (rewritten by agentguard init)
{
  "github": {
    "command": "agentguard",
    "args": ["wrap", "--upstream-name", "github", "--",
             "npx", "-y", "@modelcontextprotocol/server-github"]
  }
}
```

The original config is backed up to `~/.claude/mcp_servers.json.agentguard.bak`. `agentguard uninstall` restores it perfectly. For HTTP MCP servers, AgentGuard rewrites the URL to point at the local proxy (`http://localhost:7879/mcp/<server-name>`), which forwards to the real upstream.

The same patcher knows how to find and rewrite:
- `~/.cursor/mcp.json`
- `~/.codex/config.toml` (Codex CLI)
- `~/.config/gemini-cli/mcp.json`
- `~/.config/windsurf/mcp_servers.json`
- Project-local `.mcp.json` files (with `--project` flag)

---

## 7. Mind-Blowing Features (the "people will love this" list)

These are the features that make the launch tweet write itself. Each is genuinely useful, not gimmick.

### 7.1 Time-travel debugging (`agentguard replay`)
Every tool call is recorded with its exact request, response, and the agent's intent (extracted from the prompt context). `agentguard replay <session-id>` re-runs the session against a mock upstream, letting you debug an agent failure without rerunning the LLM. Costs $0.

### 7.2 "What is this agent doing right now?" live tail (`agentguard tail`)
A Bubble Tea TUI that shows every tool call across every connected agent in real time, with color-coded verdicts, latency, and token cost. Looks like `htop` for AI agents. People will record terminal videos and post them. **This alone is the viral moment.**

### 7.3 The Trust Score Dashboard
Each connected MCP server gets a live trust score (0–100) based on:
- Last commit date (abandonment risk)
- Schema stability (rug-pull risk)
- Auth presence (eavesdropping risk)
- Known CVEs
- Community signal (GitHub stars, issue close rate from public registry data we ingest)

This is the public-good piece — we publish trust scores at `agentguard.dev/registry` even for users not running the sidecar. Becomes the canonical "is this MCP server safe?" source.

### 7.4 One-click prompt-injection test (`agentguard scan`)
Point it at any MCP server URL or command. AgentGuard fires a battery of 50+ known prompt-injection payloads at it and reports which ones would have reached the model. Free red-team-in-a-bottle.

```bash
$ agentguard scan github
Scanning github MCP server with 53 attack patterns...
✓ Resisted indirect injection via issue body (1/53)
✗ Vulnerable to tool-description override (2/53) ← CVE-CANDIDATE
✓ Resisted base64-encoded payload (3/53)
...
Result: 51/53 passed. 2 failures. Full report → http://localhost:7878/scan/abc123
```

### 7.5 Spend caps that actually work
`agentguard policy set --tool=bash --max-cost-usd=2.00 --per-session` — and the sidecar enforces it. Agent tries to exceed it → blocked with a clean error message the agent can read and reason about.

### 7.6 "Was that intentional?" — Intent attestation
Before allowing a destructive tool call (`rm`, `DROP TABLE`, `POST /transfer`), AgentGuard captures the most recent prompt and asks the agent (via a structured tool-result) to confirm in one word whether this matches the user's intent. Catches the "agent hallucinated a DROP TABLE" failure mode that breaks production.

### 7.7 Rule packs (community-shareable policy bundles)
A rule pack is a YAML file that codifies a set of policies (allowed tools, blocked patterns, spend caps). We ship official packs:
- `agentguard pack install fintech` — PCI-friendly defaults
- `agentguard pack install opensource-maintainer` — what a solo OSS dev needs
- `agentguard pack install hackathon` — permissive, dev-friendly

Anyone can publish a pack to `agentguard.dev/packs`. This is the long-tail growth engine — every community niche makes their own pack and shares it.

### 7.8 Native skill — `~/.claude/skills/agentguard/SKILL.md`
We ship a Claude Code skill that teaches Claude *itself* to consult AgentGuard. When Claude is about to call a sensitive tool, the skill fires and tells Claude to check the policy first via `agentguard check <tool> <args>`. This is meta and people will love it: AgentGuard isn't just protecting the agent from outside, it's enrolling the agent in its own defense.

### 7.9 Offline mode (`agentguard --air-gapped`)
For regulated industries: ML model is bundled, no network calls for inspection, all telemetry stays on disk. This unlocks government / healthcare / finance buyers later.

### 7.10 Per-project policies
Drop a `.agentguard.yaml` in any project root and AgentGuard picks it up automatically. Lets a monorepo enforce different rules per directory.

---

## 8. Performance Budget

| Metric | Target | Rationale |
|--------|--------|-----------|
| Cold start (sidecar boot) | < 200 ms | Below human perception |
| Stage 0–4 latency (cheap path) | < 2 ms p99 | Imperceptible per call |
| Stage 5 (ML, hot path) | < 50 ms p99 | Only invoked on suspicious traffic |
| Full proxy roundtrip overhead | < 5 ms p99 (cheap path) | Below MCP server's own latency |
| Memory at idle | < 80 MB RSS | Comfortable on a laptop |
| Memory under load (1000 req/s) | < 250 MB RSS | Server-grade headroom |
| Binary size | < 25 MB | One-shot download stays snappy |
| Dashboard bundle | < 200 KB gzip | Loads instantly even on cellular |

Bench targets are validated in CI on every PR via `bench/` suite. Regressions block merge.

---

## 9. Trust & Threat Model

### 9.1 What AgentGuard defends against
| Threat | Defense |
|--------|---------|
| Indirect prompt injection in tool results | Stages 4 + 5 of pipeline |
| Tool poisoning (silent schema rug-pull) | Stage 2 attestation |
| Abandoned/CVE-laden MCP servers | Stage 2 + Trust Score |
| Infinite loops / runaway cost | Stage 6 circuit breaker |
| Credential exfiltration through tool output | Stage 4 secret scanner |
| Unauthenticated MCP servers | Transport guard + alert |
| Sampling-based attacks (Unit 42 vector) | Sampling requests routed through policy |
| Destructive tool calls without consent | Intent attestation (7.6) |

### 9.2 What AgentGuard explicitly does NOT defend against
- The LLM itself being misaligned (that's Anthropic's problem)
- Network-layer attacks on the developer's machine (use a firewall)
- Physical compromise of the developer's machine (use disk encryption)
- The user explicitly choosing to disable AgentGuard

We are honest about the boundaries. Marketing this as "stops all AI attacks" would be lying and would get us roasted on Hacker News.

### 9.3 AgentGuard's own threat surface
- Signed binaries via Cosign, hashes published on GitHub Releases
- Reproducible builds (Go + locked deps → bit-identical binaries)
- Zero network calls in default-local mode (verifiable with `tcpdump`)
- All dashboard endpoints bound to `127.0.0.1` by default
- SQLite database is mode 0600 in `~/.agentguard/`
- Cloud tier is optional, opt-in, and uses end-to-end encrypted audit-log batches (keys never leave the device)

---

## 10. Concurrency & Process Model

```
                                    ┌───────────────────────────────┐
                                    │  agentguard daemon (systemd /  │
                                    │  launchd / Task Scheduler /    │
                                    │  user-mode background)         │
                                    └────────────────┬──────────────┘
                                                     │
                                                     ├── spawns ──┐
                                                     │            ▼
                                                     │  ┌────────────────────────┐
                                                     │  │  HTTP listener :7879   │
                                                     │  │  (proxied MCP/A2A)     │
                                                     │  └────────────────────────┘
                                                     │
                                                     ├── spawns ──┐
                                                     │            ▼
                                                     │  ┌────────────────────────┐
                                                     │  │  Dashboard HTTP :7878  │
                                                     │  │  (SSE + static assets) │
                                                     │  └────────────────────────┘
                                                     │
                                                     └── spawns N x ─┐
                                                                     ▼
                                                       ┌────────────────────────┐
                                                       │  stdio wrappers        │
                                                       │  (one per MCP server   │
                                                       │   invoked via `wrap`)  │
                                                       └────────────────────────┘
```

- The daemon supervises everything; if a wrapper crashes the parent agent gets a clean error rather than a hang.
- Inspection pipeline runs as goroutines per request — no blocking.
- DB writes are batched via a channel-fed writer goroutine; 10ms or 100 events, whichever comes first.

---

## 11. Open Source & Monetization Strategy

### 11.1 Licensing
- **Core sidecar, CLI, dashboard, all detection logic**: MIT
- **Rule packs marketplace**: free, anyone can publish
- **AgentGuard Cloud (Pro)**: source-available (BSL 1.1 → MIT after 3 years), commercial

### 11.2 Free forever
- Everything a solo dev or hobbyist needs
- No telemetry phone-home (explicitly verifiable)
- No login required to use
- Community rule packs

### 11.3 Pro ($19/dev/month or $99/team/month)
- Centralized policy sync across machines
- Team audit log with retention
- SSO / RBAC
- Slack/PagerDuty alerts on blocked calls
- SLA support, priority CVE response
- Anthropic Verified plugin badge

### 11.4 Enterprise (custom)
- Self-hosted control plane
- HIPAA / SOC2 attestations
- Custom rule pack development
- Dedicated CVE early-warning feed

### 11.5 GitHub Sponsors funnel
The README has a prominent "Sponsor this project" with three concrete tiers ($5 / $25 / $250) and what each unlocks (early access to rule packs, named in release notes, founder office hours).

---

## 12. Launch Plan (first 14 days)

1. **Day -7 to -1:** Polish landing page, write three launch blog posts:
   - "I scanned 1,847 MCP servers. 52% are abandoned. Here's the firewall." (the data piece)
   - "Your AI agent can be hijacked by a GitHub issue. Here's a 10-second fix." (the demo piece)
   - "Why I built AgentGuard from Nepal in 6 weeks." (the founder piece)
2. **Day 0:** Post to Hacker News at 8am PT Tuesday. Cross-post to r/LocalLLaMA, r/ClaudeAI, r/AINativeDev, Lobsters, Lemmy.
3. **Day 0:** Tweet thread with a 45-second screen recording of `agentguard tail` catching a real injection attempt.
4. **Day 1-3:** Engage every comment, ship 2 bug fixes a day, post the changelog publicly.
5. **Day 4-7:** Reach out to creators (Theo, Fireship, Matt Pocock, Boris Cherny if possible) with a personalized demo.
6. **Day 7-14:** Apply for the Anthropic Partner Network and Anthropic Verified plugin badge.

---

## 13. What's NOT in v1 (deliberately deferred)

- Multi-language SDK (Go core + npm wrapper is enough)
- Browser extension for Claude.ai web (different attack surface, separate project)
- Custom rule-pack signing infrastructure (manual review for v1)
- Mobile app for the dashboard (the dashboard is responsive; native can wait)
- Self-hosted Cloud tier (Cloudflare hosted only until we have 50 paying teams)

Scope discipline is how this ships in 6 weeks instead of 6 months.
