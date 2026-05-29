# AgentGuard — Requirements & Build Specification

> **Audience:** Claude Code, executing this document as a multi-week implementation plan.
> **Tone:** Imperative. Every requirement is a directive Claude Code should treat as binding.
> **Reading order:** Read `system-design.md` and `database.md` first. This document operationalizes both.

---

## 0. Project North Star

Build a single-binary, MIT-licensed, zero-config security sidecar for AI agents that:

1. **Installs in one shell command** and auto-wires itself into Claude Code, Cursor, Codex, Gemini CLI, and Windsurf within 10 seconds of the user pressing Enter.
2. **Proxies MCP and A2A traffic** between agents and their tools/peers without modifying either protocol.
3. **Detects and blocks** prompt injection, tool poisoning, rug-pulls, runaway loops, and credential exfiltration before they reach the model or the network.
4. **Surfaces everything** through a beautiful real-time dashboard (`localhost:7878`) and a TUI live-tail (`agentguard tail`).
5. **Stays out of the way** until something interesting happens, then makes that interesting thing easy to investigate and replay.

If a feature compromises any of those five, cut the feature.

---

## 1. Repository Layout

```
agentguard/
├── README.md                          # Marketing-grade, with a 30-sec GIF
├── LICENSE                            # MIT
├── CODE_OF_CONDUCT.md
├── CONTRIBUTING.md
├── SECURITY.md                        # responsible disclosure address
├── CHANGELOG.md                       # Keep-a-Changelog format
├── .github/
│   ├── workflows/
│   │   ├── ci.yml                     # test, lint, build matrix
│   │   ├── release.yml                # GoReleaser, Cosign signing
│   │   ├── bench.yml                  # performance regression check
│   │   └── docs.yml                   # deploys docs site
│   ├── ISSUE_TEMPLATE/
│   ├── FUNDING.yml                    # GitHub Sponsors
│   └── dependabot.yml
├── cmd/
│   └── agentguard/                    # CLI entrypoint (cobra root)
│       └── main.go
├── internal/                          # all private packages (Go convention)
│   ├── proxy/                         # MCP/A2A interception
│   │   ├── stdio.go                   # stdio transport
│   │   ├── streamable_http.go         # streamable-HTTP transport
│   │   ├── sse.go                     # legacy SSE
│   │   └── router.go                  # which inspection chain to run
│   ├── pipeline/                      # inspection stages
│   │   ├── stage.go                   # interface + chain runner
│   │   ├── transport.go               # stage 0
│   │   ├── schema.go                  # stage 1
│   │   ├── attestation.go             # stage 2
│   │   ├── policy.go                  # stage 3
│   │   ├── scanner.go                 # stage 4 (regex/heuristics)
│   │   ├── ml.go                      # stage 5 (ONNX classifier)
│   │   └── circuit.go                 # stage 6
│   ├── policy/                        # OPA-style evaluation engine
│   │   ├── engine.go
│   │   ├── compiler.go                # YAML/Rego -> AST
│   │   └── packs.go
│   ├── registry/                      # trust score, abandonment check
│   │   ├── github.go                  # repo metadata fetcher
│   │   ├── npm.go
│   │   ├── pypi.go
│   │   └── score.go
│   ├── ml/                            # ONNX runner
│   │   ├── classifier.go
│   │   └── models.go                  # model loading/caching
│   ├── store/                         # SQLite + DuckDB
│   │   ├── sqlite.go
│   │   ├── duckdb.go
│   │   ├── batched_writer.go
│   │   ├── retention.go
│   │   └── migrations/                # goose .sql files
│   ├── dashboard/                     # HTTP server for SvelteKit app
│   │   ├── server.go
│   │   ├── sse.go                     # real-time event stream
│   │   ├── api.go                     # JSON REST endpoints
│   │   └── embed.go                   # embeds /web/dist via embed.FS
│   ├── cli/                           # cobra command tree
│   │   ├── root.go
│   │   ├── init.go
│   │   ├── wrap.go                    # the magic `agentguard wrap` cmd
│   │   ├── tail.go                    # Bubble Tea TUI
│   │   ├── scan.go
│   │   ├── replay.go
│   │   ├── pack.go
│   │   ├── policy.go
│   │   ├── config.go
│   │   ├── doctor.go
│   │   ├── uninstall.go
│   │   └── version.go
│   ├── agent_detect/                  # finds installed Claude Code, Cursor, etc.
│   │   ├── claude_code.go
│   │   ├── cursor.go
│   │   ├── codex.go
│   │   ├── gemini.go
│   │   ├── windsurf.go
│   │   └── patcher.go                 # rewrites config files safely
│   ├── otel/                          # OpenTelemetry exporter
│   │   └── exporter.go
│   ├── crypto/                        # signing, attestation helpers
│   │   ├── cosign.go
│   │   └── hashing.go
│   ├── daemon/                        # background process supervisor
│   │   ├── supervisor.go
│   │   ├── launchd.go                 # macOS
│   │   ├── systemd.go                 # Linux
│   │   └── windows_svc.go             # Windows
│   └── version/
│       └── version.go                 # injected via -ldflags
├── pkg/                               # public API surface (for npm wrapper)
│   └── client/
│       └── client.go
├── npm/                               # @agentguard/cli wrapper
│   ├── package.json
│   ├── bin/agentguard.js
│   ├── install.js                     # postinstall: pulls correct binary
│   └── README.md
├── web/                               # SvelteKit dashboard
│   ├── package.json
│   ├── svelte.config.js
│   ├── vite.config.ts
│   ├── tailwind.config.ts
│   ├── src/
│   │   ├── routes/
│   │   │   ├── +layout.svelte
│   │   │   ├── +page.svelte           # overview
│   │   │   ├── calls/+page.svelte
│   │   │   ├── servers/+page.svelte
│   │   │   ├── policies/+page.svelte
│   │   │   ├── packs/+page.svelte
│   │   │   ├── replay/[id]/+page.svelte
│   │   │   └── settings/+page.svelte
│   │   ├── lib/
│   │   │   ├── api.ts
│   │   │   ├── sse.ts
│   │   │   └── components/...
│   │   └── app.css
│   └── static/
├── packs/                             # builtin rule packs (YAML)
│   ├── default.yaml
│   ├── strict.yaml
│   ├── fintech.yaml
│   ├── opensource-maintainer.yaml
│   └── hackathon.yaml
├── models/                            # ONNX models shipped in releases
│   └── injection-classifier-v1.onnx
├── install/                           # install scripts
│   ├── install.sh                     # POSIX
│   ├── install.ps1                    # Windows
│   └── homebrew/                      # tap formula
├── docs/                              # mdbook source
│   ├── book.toml
│   └── src/...
├── bench/                             # criterion-style benchmarks
│   ├── proxy_bench_test.go
│   ├── pipeline_bench_test.go
│   └── store_bench_test.go
├── e2e/                               # end-to-end tests
│   ├── mock_mcp_server/
│   ├── claude_code_test.sh
│   └── ...
├── skill/                             # the Claude Code skill we ship
│   └── SKILL.md
├── go.mod
├── go.sum
├── Makefile
├── .goreleaser.yaml
└── docker-compose.yml                 # for local dev: agentguard + mock servers
```

**Hard rule:** no nested module (`go.mod`) anywhere except `npm/` (which is a JS package). The repo is one Go module.

---

## 2. Tech Stack (Locked Choices)

| Layer | Choice | Version (May 2026) |
|-------|--------|-------------------|
| Go | Go | 1.23.x |
| HTTP server | net/http (stdlib) | — |
| Routing | chi/v5 | latest |
| JSON-RPC | sourcegraph/jsonrpc2 | latest |
| CLI framework | spf13/cobra | latest |
| TUI | charmbracelet/bubbletea + bubbles + lipgloss | latest |
| Config | spf13/viper + koanf for YAML | latest |
| SQLite | modernc.org/sqlite | latest |
| DuckDB | marcboeker/go-duckdb | latest |
| Migrations | pressly/goose | latest |
| Logging | log/slog (stdlib) | — |
| ULID | oklog/ulid/v2 | latest |
| OTLP | go.opentelemetry.io/otel | latest |
| ONNX | yalue/onnxruntime_go (or wasmer-go fallback) | latest |
| YAML | goccy/go-yaml | latest |
| Crypto signing | sigstore/cosign (cmd-line, called via shell) | latest |
| HTTP testing | net/http/httptest | — |
| Dashboard runtime | Bun | 1.2.x |
| Dashboard framework | SvelteKit | 2.x |
| Styling | Tailwind CSS | 4.x |
| Charts | Apache ECharts (via echarts-svelte) | latest |
| Icons | lucide-svelte | latest |
| Release | goreleaser/goreleaser | latest |
| Test framework (Go) | stretchr/testify | latest |
| Test framework (web) | Playwright + Vitest | latest |
| Pre-commit | lefthook | latest |
| Linter | golangci-lint | latest |

**No surprises.** No new dependency lands without a justification in the PR description.

---

## 3. Functional Requirements (by command/feature)

### 3.1 `agentguard init`

**Goal:** Take the user from "binary on disk" to "everything is protected" in under 10 seconds.

Behavior:
1. Print a banner and the version.
2. Create `~/.agentguard/{bin,data,logs,packs,config}` with mode 0700.
3. Initialize the SQLite database; run all migrations.
4. Detect installed agents by scanning known config paths. Show a checklist UI (Bubble Tea) with what was found, all pre-checked.
5. For each agent: back up config, rewrite MCP entries to invoke `agentguard wrap`, verify the rewrite, write an entry to the `agents` table.
6. Install the default rule pack (`packs/default.yaml`).
7. Start the daemon (`agentguard daemon start --background`).
8. Open the dashboard in the browser (`open http://localhost:7878` on macOS, equivalent elsewhere). Best-effort; print URL if it fails.
9. Total elapsed time printed at the end: should consistently be < 10s on a warm system.

Flags:
- `--no-browser` skip opening browser
- `--no-daemon` don't start the daemon (CI/test)
- `--non-interactive` accept all detected agents, no prompts
- `--pack <name>` install a different default pack
- `--project <path>` initialize at project scope (writes `.agentguard.yaml` instead of patching global configs)

### 3.2 `agentguard wrap`

**Goal:** Transparent stdio MCP proxy. This is what the rewritten agent configs invoke.

```
agentguard wrap --upstream-name github -- npx -y @modelcontextprotocol/server-github
```

Behavior:
1. Open a session in the DB.
2. Spawn the upstream command with its stdin/stdout piped.
3. Loop: read a JSON-RPC message from one side, run the pipeline against it, forward or block.
4. On block, return a clean JSON-RPC error to the agent (`code: -32099`, `message: "Blocked by AgentGuard: <reason>"`).
5. On upstream exit, close the session row and exit with the same code.

Must handle:
- Long-lived connections (keep file handles open until the agent disconnects)
- Concurrent message multiplexing (JSON-RPC requests can interleave with responses)
- Graceful shutdown on SIGINT/SIGTERM
- Upstream crashes: report to the agent cleanly, never hang

### 3.3 `agentguard tail` — the viral TUI

**Goal:** A terminal experience so satisfying people record it and post it.

A Bubble Tea full-screen TUI showing a live, color-coded scrolling feed of every tool call across every active agent.

Columns (left to right):
- Timestamp (HH:MM:SS.mmm)
- Agent (emoji + name)
- Server (color-coded by trust score)
- Tool (with direction arrow)
- Verdict (✓ green / ✗ red / ⚠ yellow / · gray)
- Latency (ms, with color gradient)
- Cost (if non-zero)

Keybindings:
- `j/k` or arrows: navigate
- `enter`: open detail view for the selected call (full request/response, stage trail)
- `f`: filter (agent / server / verdict / tool — composable)
- `/`: search
- `p`: pause / resume
- `r`: replay selected call
- `c`: clear screen
- `?`: help overlay
- `q`: quit

The detail view shows a side-by-side diff if the call was transformed, and a per-stage timing breakdown. Built with lipgloss for the layout. Must remain responsive at 500 events/sec by sampling.

### 3.4 `agentguard scan`

**Goal:** A one-command red team for any MCP server.

```
agentguard scan <server-name-or-uri>
```

Behavior:
1. Resolve the target (an installed server name, an npm package, or a URL).
2. Spawn it in a sandboxed wrapper.
3. Run all 50+ canned attack payloads (loaded from `packs/scan-payloads.yaml`):
   - Prompt injection in tool descriptions
   - Indirect injection in tool results
   - Tool description rug-pull (description changes between invocations)
   - Schema drift (input schema changes)
   - Base64/rot13/zero-width-char obfuscated payloads
   - Sampling-request abuse (Unit 42 vector)
   - Sensitive-tool name collision (e.g., a tool that names itself `read_file` but does `exec`)
   - Resource exhaustion (returns 10MB blob)
4. Report a numeric score and a per-attack pass/fail breakdown.
5. Output a Markdown report at `~/.agentguard/scans/<server>-<timestamp>.md` and a dashboard URL.

Exit code: 0 if all pass, 1 if any fail. Useful in CI.

### 3.5 `agentguard replay`

```
agentguard replay <call-id>     # one call
agentguard replay --session <session-id>   # whole session
```

Re-runs the captured call against either (a) a mock that returns the recorded upstream response (default — costs nothing) or (b) the live upstream (with `--live` flag). The pipeline runs again. Use case: you changed a policy, you want to see how the past would have looked.

### 3.6 `agentguard pack`

```
agentguard pack list                 # show installed
agentguard pack install <slug>        # install from agentguard.dev/packs
agentguard pack uninstall <slug>
agentguard pack create <name>         # scaffolds a new pack
agentguard pack publish               # uploads to the registry (auth'd)
agentguard pack inspect <slug>        # show the rules in a pack
```

Packs are YAML files validated against a JSON schema. The installer verifies the SHA-256 against the registry's published manifest.

### 3.7 `agentguard policy`

```
agentguard policy list
agentguard policy show <name>
agentguard policy enable <name>
agentguard policy disable <name>
agentguard policy edit <name>         # opens $EDITOR
agentguard policy set <name> --tool=bash --max-cost-usd=2 --per-session
```

### 3.8 `agentguard doctor`

A health check that reports:
- All connected agents and whether their config rewrites are intact
- Daemon status
- Database integrity (PRAGMA integrity_check)
- ML model presence + checksum
- Recent crashes / errors
- Disk space in `~/.agentguard/`
- Cloud sync status (if enabled)

Output mimics `homebrew doctor` style. Each finding is either `✓`, `⚠`, or `✗` with a remediation hint.

### 3.9 `agentguard uninstall`

**Critical.** A bad uninstall kills the project's reputation.

Behavior:
1. Confirmation prompt (skippable with `--yes`).
2. Stop the daemon.
3. For each `agents` row: restore the original config from `config_backup`. Verify the restore byte-by-byte.
4. Optionally export the DB to `~/agentguard-export.tar.zst` (default: yes, with `--keep-data=false` to skip).
5. Remove `~/.agentguard/` recursively.
6. Remove the binary from `~/.agentguard/bin/`.
7. Print a thank-you and a short, sincere survey link.

If anything fails, abort and leave the system in a recoverable state. Never delete anything we can't restore.

### 3.10 Dashboard (`http://localhost:7878`)

Pages:
- `/` Overview — sparklines for calls/min, blocks/min, cost/hour. Top 5 tools, top 5 servers. Recent activity.
- `/calls` Searchable, filterable, paginated. Click → detail.
- `/calls/:id` Full request/response. Stage timeline. Replay button. "Mark as false positive" button.
- `/servers` All known MCP servers, trust scores, last seen, click for attestation history.
- `/policies` CRUD policies. YAML editor with live validation.
- `/packs` Installed packs + marketplace browser (fetches from agentguard.dev/packs).
- `/replay/:id` Compare original vs replayed run side-by-side.
- `/settings` Retention, telemetry, cloud sync.

UX rules:
- No login screen. Loads instantly.
- Dark mode default, light optional, system-aware toggle.
- All data fetched via SSE (live) or `/api/*` JSON (paginated).
- No client-side analytics, no Google fonts, no third-party assets.

---

## 4. Inspection Pipeline — Concrete Specs

### 4.1 Stage 0: Transport guard
- For HTTP transports: require `Authorization` header if the server's `auth_required = 1`.
- Local stdio: no auth needed; trust the OS user boundary.
- Rate limit: 100 req/s per session by default. Configurable.

### 4.2 Stage 1: Schema validator
- Use the canonical MCP / A2A JSON schemas (vendored in `internal/schemas/`).
- Reject anything that fails validation with a structured JSON-RPC error.

### 4.3 Stage 2: Server attestation
- On first call to a server, compute `binary_sha256` (stdio) or TLS cert fingerprint (HTTP).
- Compare `tool_name + description_hash + input_schema_hash` against `server_tools`.
- If any drift detected: block the call, write a `flag` event, surface a dashboard notification ("github.create_issue's description changed — review and approve").
- After 24 hours of no drift, auto-approve. Configurable.

### 4.4 Stage 3: Policy engine
- Evaluate rules in priority order.
- Match criteria: `server`, `tool`, `direction`, `content_match` (regex), `cost_threshold`.
- Actions: `allow`, `block`, `require_approval`, `flag`, `redact`, `rate_limit`.
- Approval prompts are surfaced via the dashboard and (if configured) via OS notification.

### 4.5 Stage 4: Content scanner
- Built-in regex bank for: AWS keys, GCP keys, GitHub tokens, Stripe keys, JWTs, private SSH keys, OpenAI keys, Anthropic keys.
- PII patterns: emails, phone numbers, SSN (US), Aadhaar (IN), credit card (Luhn-validated).
- Known prompt-injection signatures: "ignore previous instructions", "you are now", etc. (the cheap stuff — the ML stage catches the smart stuff.)
- On match: redact in the forwarded payload, store the unredacted version in `tool_call_artifacts` with kind `redacted_diff`, flag the call.

### 4.6 Stage 5: ML classifier
- Triggered when Stage 4 finds anomalies OR the response is > 4 KB OR the source is an inbound tool result.
- Run the ONNX model on CPU. Cache classifications by content hash (15-min TTL) — many calls return identical content.
- Threshold 0.85 confidence → block + flag. Configurable.
- Falls back to allow + flag if the model isn't loaded (graceful degradation).

### 4.7 Stage 6: Circuit breaker
- Detect: same tool + same args called ≥ 5 times in 60s by the same session → block with reason `loop_detected`.
- Per-session budget enforcement: hard cap on `total_cost_usd` and `total_tokens`.
- Per-tool budget enforcement.

### 4.8 Stage 7: Audit & forward
- Event written to DB (batched).
- OTLP span emitted (if exporter configured).
- Message forwarded to its destination.

---

## 5. Non-Functional Requirements

### 5.1 Performance
- Bench targets from system design (Section 8) are CI gates. Regressions > 10% fail the build.
- The dashboard never blocks the proxy: dashboard reads from a DuckDB attachment and SSE channels, never holds a write lock.

### 5.2 Reliability
- Daemon crash must not lose more than 100ms of buffered events (the batch window).
- DB writes use WAL + fsync NORMAL. Catastrophic OS crash may lose the current batch only.
- `agentguard doctor` must detect a corrupt DB and offer one-command recovery from the most recent backup.

### 5.3 Security
- All Go dependencies pinned by checksum (`go.sum`).
- `govulncheck` runs in CI on every PR.
- Releases signed with Cosign keyless (GitHub OIDC) and published to Sigstore Rekor.
- Reproducible builds: bit-identical binaries from the same git SHA.
- No telemetry by default. `--telemetry=anonymous` is opt-in only and well-documented.

### 5.4 Cross-platform
- Build matrix: `darwin/amd64`, `darwin/arm64`, `linux/amd64`, `linux/arm64`, `windows/amd64`.
- Smoke tests run on all platforms via GitHub Actions.
- No platform-specific code in business logic — only in `daemon/launchd.go`, `daemon/systemd.go`, `daemon/windows_svc.go`.

### 5.5 Accessibility (dashboard)
- WCAG 2.1 AA contrast in both themes.
- All interactions reachable by keyboard.
- ARIA labels on every dynamic region.
- Reduced-motion preference respected.

### 5.6 Internationalization
- The CLI is English-only at launch.
- Dashboard supports `en` and `ne` (Nepali — yes, your home audience matters) at launch. i18next-style JSON files for future locales.

### 5.7 Documentation
- `docs/` is mdBook, deployed to `agentguard.dev/docs` via GitHub Pages.
- Every public-facing command has a complete `--help` plus a docs page with at least one runnable example.
- A "Cookbook" section with 10 recipes (block all `rm -rf`, enforce per-project spend caps, alert on new MCP servers, etc.).

---

## 6. Open-Source Hygiene

| File | Required content |
|------|------------------|
| `README.md` | Hero line, 30-second GIF, install command, 3 highlight features, link to docs, sponsor box, badges (CI, version, downloads, license, stars) |
| `LICENSE` | MIT, copyright Nikhil [Last Name] |
| `CODE_OF_CONDUCT.md` | Contributor Covenant v2.1 |
| `CONTRIBUTING.md` | Dev setup, PR process, commit style (Conventional Commits) |
| `SECURITY.md` | `security@agentguard.dev`, GPG key, response SLA (3 business days), bug bounty tiers |
| `CHANGELOG.md` | Keep-a-Changelog, every release |
| `.github/FUNDING.yml` | `github: [your-handle]` |
| `.github/workflows/ci.yml` | Lint + test + cross-build on every PR |
| `.github/ISSUE_TEMPLATE/*.yml` | Bug, feature, security (security redirects to email) |

GitHub repo settings:
- Branch protection on `main`: requires PR + 1 review (you for now) + green CI.
- Sponsors enabled, three tiers ($5 / $25 / $250).
- Discussions enabled.
- Wiki disabled (use mdBook).

---

## 7. Testing Strategy

| Level | Coverage target | Tooling |
|-------|----------------|---------|
| Unit (Go) | ≥ 80% line coverage on `internal/pipeline/*` and `internal/policy/*` | `go test`, `testify` |
| Integration | All CLI commands invoked against a mock MCP server | shell scripts under `e2e/` |
| End-to-end | Real Claude Code, real Cursor (latest stable) in a clean container | GitHub Actions matrix |
| Performance | Bench suite under `bench/`, CI fails on 10% regression | `go test -bench` |
| Fuzz | Inspection pipeline + JSON-RPC parser fuzzed for 5 min per CI run | `go test -fuzz` |
| Dashboard | Component + a11y tests | Vitest + Playwright |
| Security | `govulncheck`, `gosec`, `gitleaks` on every PR | their respective Actions |

A PR cannot land if any of: CI red, coverage drop > 1%, new dep added without sign-off, security scanner finding above LOW.

---

## 8. The Skill We Ship (`skill/SKILL.md`)

This is the meta-touch — AgentGuard ships its own Claude Code skill.

```markdown
---
name: agentguard
description: "Use when the user wants to check, audit, or change agent-security settings, install AgentGuard, view recent blocks, scan an MCP server, manage policies or rule packs, or investigate a flagged tool call. Triggers include any mention of agent security, MCP safety, prompt injection, tool poisoning, blocked tool calls, or the agentguard CLI."
---

# AgentGuard skill

If the user wants to inspect what their agent has been doing or change a security policy:

1. Run `agentguard doctor` first to surface any issues.
2. For audit questions, query `~/.agentguard/data/agentguard.db` read-only with the schema in `~/.agentguard/data/SCHEMA.md` (auto-generated by the daemon).
3. For policy changes, use `agentguard policy ...` commands, never edit YAML directly.
4. Before suggesting a destructive change (uninstall, policy disable), confirm with the user.

The full CLI reference is at `agentguard --help` and `https://agentguard.dev/docs/cli`.
```

Yes, AgentGuard teaches Claude how to use AgentGuard. People will love this.

---

## 9. Release Engineering

- Version scheme: SemVer. `v0.x.y` until API stable.
- GoReleaser produces binaries for all platforms + checksums + a Homebrew formula PR + a Scoop manifest PR + npm package update.
- Cosign keyless signing on every release.
- Auto-generated SBOM in CycloneDX format.
- Release notes generated from Conventional Commits using `git-cliff`.

---

## 10. Cloud (deferred but designed)

When ready (post-launch, paying users asking for it):
- Cloudflare Workers + D1 + R2.
- Clerk for auth.
- Stripe for billing.
- Encrypted audit-log ingest endpoint.
- Policy sync API.
- Public rule-pack registry at `agentguard.dev/packs`.

The schema is designed so cloud sync can be added without local migration.

---

## 11. Milestones (6-week build plan)

| Week | Focus | Deliverable |
|------|-------|-------------|
| 1 | Core proxy + stdio transport | `wrap` works against a real MCP server, calls are logged to SQLite |
| 2 | Pipeline stages 0–4 + policy engine | Cheap-path inspection works, regex scanner catches secrets |
| 3 | CLI (`init`, `tail`, `scan`, `doctor`) + agent detection + auto-patching | True 10-second install on macOS |
| 4 | Dashboard MVP (overview + calls + servers pages) + SSE | Visually polished, demoable |
| 5 | ML stage + circuit breaker + replay + rule packs | Feature-complete for launch |
| 6 | Polish, docs, landing page, cross-platform testing, signing pipeline, launch | Ship to HN at 8am PT Tuesday |

Each week ends with a public devlog post on the repo's Discussions page. Build in public.

---

## 12. Out of Scope (v1)

Explicitly NOT building in v1 — say no to all of these even if a user asks:

- Browser extension for Claude.ai
- Native mobile dashboard app
- Self-hosted cloud control plane
- Custom ML model training UI
- Plugin system for third-party pipeline stages
- Windows service installer (use scoop / manual for now)
- Anything that requires running as root

These can come in v0.2+ once we know who the real users are.

---

## 13. The Definition of Done (launch criteria)

Ship to Hacker News only when ALL of these are true:

- [ ] `curl -fsSL agentguard.dev/install | sh` works on a fresh macOS, Linux, and (via Powershell variant) Windows
- [ ] On a fresh `claude-code` install, `agentguard init` finishes in < 10s and Claude Code's next tool call is intercepted
- [ ] A clean Cursor install survives `agentguard init` and uninstall round-trip (config restored byte-identical)
- [ ] `agentguard tail` looks great on a 1080p terminal screen recording
- [ ] `agentguard scan github` catches at least one real, reproducible attack
- [ ] Dashboard loads in < 1s, dark mode is beautiful, every page works
- [ ] `agentguard uninstall` leaves no trace except the optional export
- [ ] CI is green across all 5 platforms
- [ ] Bench suite shows < 5ms p99 overhead on cheap path
- [ ] Docs site is live and complete for every command
- [ ] README has a 30-second GIF, 3 testimonials (recruit beta users), and a sponsor box
- [ ] Three launch blog posts are drafted and ready
- [ ] HN/Reddit/Twitter posts are queued in Buffer
- [ ] Sponsor tiers are configured
- [ ] You've slept the night before launch

---

## 14. How Claude Code Should Execute This

Given to Claude Code as a single prompt (see `prompt-for-claude-code.md`), the expected behavior is:

1. Read this file, `system-design.md`, and `database.md` end to end.
2. Scaffold the repo per Section 1 (empty files, correct structure, license, README).
3. Implement Week 1 (Section 11): core proxy, stdio transport, SQLite store with migrations, the `wrap` command, basic logging.
4. Run the bench suite. Iterate until p99 < 5ms.
5. Commit per Conventional Commits. One feature per commit.
6. Open a PR with a checklist of what's done vs what's next.
7. Pause for human review.
8. Continue with Week 2 after approval.

Do not skip ahead. Do not introduce new tech that isn't in Section 2. If a requirement here conflicts with reality during implementation, raise it, do not silently change it.

---

## 15. Success Metrics (Day 30 post-launch)

- ≥ 5,000 GitHub stars
- ≥ 500 unique installs (measured by daemon's anonymized first-run ping, opt-in)
- ≥ 20 third-party rule packs published
- ≥ 1 Hacker News front page appearance
- ≥ 3 mentions by AI-dev creators (Theo, Fireship, Matt Pocock, Boris Cherny equivalents)
- ≥ 10 GitHub Sponsors
- ≥ 1 production-user case study (a small company saying "we run this in CI now")
- Zero critical CVEs in AgentGuard itself

If we hit these, the Pro tier rollout (Month 2) has an audience. If we don't, we iterate based on what the user feedback actually said.
