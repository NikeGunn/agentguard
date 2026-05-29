# AgentGuard — Repository Audit

> Generated as Phase 0 of the completion run. Audited against `requirements.md`,
> `system-design.md`, and `database.md` (now vendored into the repo root).
> Branch: `feat/complete-stubs-and-verify`. Baseline commit: `47427fc`.

## Executive summary

The repository is **substantially complete and green**: `go build ./...` succeeds,
all 25 Go test packages pass, the embedded real-time dashboard exists and wires an
`EventSource` to the Go SSE stream, and the `agentguard.dev → agentguard.space`
domain migration is **already done** (the only remaining `.dev` strings live in
`CHANGELOG.md` where they *describe* the migration).

The concrete remaining gap is a set of **15 spec-listed files that ship as 3-line
package-declaration stubs**. The running binary implements equivalent behaviour
inline today (e.g. tool-schema hashing lives in `pipeline/attestation.go`, the
analytics live in `store/analytics.go`), so the stubs are not on any hot path —
but the spec (Section 1 repo layout) calls for them to exist as real, tested
modules. This run fills all 15 with production-grade, DRY implementations + tests.

---

## 1. File status table

Status legend: `complete` = implemented + (where applicable) tested; `stub` =
placeholder package decl only; `partial` = real but shallower than spec; `scaffold`
= present but not the shipping implementation.

### `internal/` — core (Go)

| File | Status | Notes |
|------|--------|-------|
| `proxy/stdio.go` | complete | Transparent stdio MCP proxy, pipeline-wired, session/tool_call recording. |
| `proxy/router.go` | **stub** | To implement: transport→chain selection. |
| `proxy/sse.go` | **stub** | To implement: legacy SSE transport. |
| `proxy/streamable_http.go` | **stub** | To implement: streamable-HTTP transport. |
| `pipeline/stage.go` | complete | Chain runner + Stage interface. Tested. |
| `pipeline/transport.go` | complete | Stage 0 (rate limit / auth). Tested. |
| `pipeline/schema.go` | complete | Stage 1 JSON-RPC validation. Tested. |
| `pipeline/attestation.go` | complete | Stage 2 rug-pull detection. Tested. Holds inline tool-schema hashing → will reuse `crypto`. |
| `pipeline/policy.go` | complete | Stage 3 policy eval bridge. |
| `pipeline/scanner.go` | complete | Stage 4 regex/secret/PII scan. Tested. |
| `pipeline/ml.go` | complete | Stage 5 classifier bridge. |
| `pipeline/circuit.go` | complete | Stage 6 loop/budget breaker. Tested. |
| `pipeline/detail.go` | complete | Human-readable verdict-reason decoding. |
| `policy/engine.go` | complete | OPA-style evaluation. Tested. |
| `policy/compiler.go` | complete | YAML→AST. Tested. |
| `policy/packs.go` | complete | Pack loading. |
| `registry/{github,npm,pypi,score}.go` | complete | Trust-score fetchers. `score` tested. |
| `ml/classifier.go` | complete | Feature-based injection classifier. Tested. |
| `ml/models.go` | **stub** | To implement: model loader/cache + checksum (doctor needs it). |
| `store/sqlite.go` | complete | Schema, migrations, pragmas, writer. Tested. |
| `store/batched_writer.go` | complete | Batched insert goroutine (database.md §5). |
| `store/analytics.go` | complete | Dashboard read queries. |
| `store/calldetail.go` | complete | Single-call detail. Tested. |
| `store/attestation.go` | complete | Server-hash get/set. |
| `store/broadcast.go` | complete | SSE broadcaster. |
| `store/duckdb.go` | **stub** | To implement: read-only DuckDB attach w/ graceful fallback. |
| `store/retention.go` | **stub** | To implement: nightly retention/vacuum (database.md §7). |
| `dashboard/{server,api,sse,embed}.go` | complete | Live dashboard + SSE + REST. Tested. |
| `cli/{root,wrap,version,init,init_tui,uninstall,doctor,tail,scan,replay,pack,seed,seed_traffic,dashboard,paths}.go` | complete | Full command tree. Several tested. |
| `cli/config.go` | **stub** | To implement: `config get/set/list` + wire into root. |
| `cli/policy.go` | **stub** | To implement: `policy list/show/enable/disable/set` + wire into root. |
| `agent_detect/*.go` | complete | Claude/Cursor/Codex/Gemini/Windsurf detection + patcher. Tested. |
| `otel/exporter.go` | complete | OTLP exporter. Tested. |
| `crypto/hashing.go` | **stub** | To implement: SHA-256 / canonical-JSON / tool-schema hashing. |
| `crypto/cosign.go` | **stub** | To implement: cosign verify wrapper, graceful degrade if absent. |
| `daemon/supervisor.go` | **stub** | To implement: process supervisor (start/stop/status/pidfile). |
| `daemon/launchd.go` | **stub** | To implement: macOS plist generation. |
| `daemon/systemd.go` | **stub** | To implement: Linux unit generation. |
| `daemon/windows_svc.go` | **stub** | To implement: Windows service descriptor. |
| `version/version.go` | complete | ldflags-injected version. |
| `pkg/client/client.go` | **stub** | To implement: public read-only client over the dashboard API. |

### Non-Go

| Path | Status | Notes |
|------|--------|-------|
| `web/**` (SvelteKit) | scaffold | Only `+layout`/`+page` + config. The **shipping** dashboard is the embedded vanilla HTML/JS/CSS under `internal/dashboard/assets/` (1167 lines, live SSE). Spec Section 1 lists SvelteKit; the embedded build satisfies the *behaviour* spec (Section 3.10) and the bundle budget. Logged in OPEN_QUESTIONS.md. |
| `internal/dashboard/assets/{index.html,app.js,app.css}` | complete | The real live dashboard. |
| `store/migrations/0001_initial.sql` | complete | Full 13-table schema from database.md in one migration. |
| `packs/{default,strict,fintech,opensource-maintainer,hackathon}.yaml` | complete | All five builtin packs present. |
| `install/{install.sh,install.ps1,homebrew/}` | complete | Domain = agentguard.space. |
| `demo/agentguard.tape` + `demo/agentguard.gif` | partial | One tape/gif. Phase 7 adds install/tail/scan/doctor tapes + dashboard recorder. |
| `docs/` (mdBook) | complete | book.toml + src. |
| `skill/SKILL.md` | complete | Shipped Claude skill. |
| `.github/workflows/*` | complete | ci/release/bench/docs. |

---

## 2. requirements.md Section 3 feature map

| Feature | Status | Evidence |
|---------|--------|----------|
| 3.1 `init` (dirs, migrate, detect, patch, daemon, browser) | done | `cli/init.go` + `init_tui.go`; agent_detect + patcher tested. |
| 3.2 `wrap` (stdio proxy) | done | `proxy/stdio.go`, e2e tests pass. |
| 3.3 `tail` (Bubble Tea TUI) | done | `cli/tail.go`. |
| 3.4 `scan` (attack battery + MD report) | partial | `cli/scan.go` exists; `packs/scan-payloads.yaml` to verify/expand in Phase 4. |
| 3.5 `replay` (mock + `--live`) | done | `cli/replay.go`. |
| 3.6 `pack` (list/install/uninstall/create/inspect) | done | `cli/pack.go`, tested. |
| 3.7 `policy` (list/show/enable/disable/set) | **missing** | `cli/policy.go` is a stub, not wired into root. **This run.** |
| 3.8 `doctor` (health checklist) | done | `cli/doctor.go`; gains ML-model checksum check once `ml/models.go` lands. |
| 3.9 `uninstall` (restore byte-identical) | done | `cli/uninstall.go`. |
| 3.10 Dashboard (8 pages, SSE, dark mode) | partial | Embedded live dashboard with SSE + overview/calls/servers; deeper per-page polish tracked Phase 3. |
| Pipeline stages 0–7 | done | `internal/pipeline/*` all implemented + tested. |
| `config` command (3.x / §3.8 cloud-sync, retention) | **missing** | `cli/config.go` stub. **This run.** |
| DuckDB analytics attach (§6 read path) | **missing** | `store/duckdb.go` stub. **This run.** |
| Retention/vacuum job (§7) | **missing** | `store/retention.go` stub. **This run.** |
| Daemon supervisor + service units (§10) | **missing** | `daemon/*` stubs. **This run.** |
| crypto signing/hashing helpers (§5.3, §4.3) | **missing** | `crypto/*` stubs. **This run.** |
| ML model loader/cache (§4.6) | **missing** | `ml/models.go` stub. **This run.** |
| Public client API (npm wrapper surface) | **missing** | `pkg/client/client.go` stub. **This run.** |

---

## 3. `agentguard.dev` reference proof

Migration is **complete**. A whole-repo grep (excluding `.git/`, `node_modules/`,
stale `.claude/worktrees/`, and build artifacts) finds references **only** inside
`CHANGELOG.md`, where they document the migration itself:

```
./CHANGELOG.md:38:- Migrated the final `agentguard.dev` reference (the VHS demo tape install
./CHANGELOG.md:39:  line) to `agentguard.space`; the repo now has zero `agentguard.dev`
```

Note: `requirements.md`/`system-design.md`/`database.md` (just vendored in) contain
historical `agentguard.dev` strings as part of the original spec text. These are
**spec documents, not shipping code/config**, and are intentionally preserved
verbatim as the historical record. No code, install script, manifest, dashboard
asset, or doc-site page references the dead domain.

Phase 1 (domain migration) is therefore **already satisfied** for all
shipping surfaces.

---

## 4. Out-of-scope confirmations (requirements.md §12)

Not built, by design: browser extension, native mobile dashboard, self-hosted
control plane, ML-training UI, third-party pipeline-stage plugins, Windows
*service installer* (a service *descriptor* generator is in scope per §10 daemon),
anything requiring root. This run does not add any of these.
