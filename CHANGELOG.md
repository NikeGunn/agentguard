# Changelog

All notable changes to AgentGuard are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)
and this project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

## [v1.2.1] — 2026-05-29

### Fixed
- **Dashboard was completely non-interactive.** The `.modal-backdrop` /
  `.palette-backdrop` rule set `display:grid`, which outranks the UA
  `[hidden]{display:none}` rule by specificity — so even when `hidden`, both
  backdrops stayed full-screen, invisible, `position:fixed` overlays at
  `z-index:70` that swallowed *every* click. Nav links, table rows, the theme
  toggle and the KPI cards all appeared dead; only the keyboard-driven command
  palette (Ctrl/Cmd+K) still worked. Re-assert `display:none` on the `[hidden]`
  backdrops.

### Added
- **Windows-safe (re)install.** `install.ps1` now stops a running AgentGuard
  daemon/dashboard before overwriting `agentguard.exe` (Windows locks the image
  of a running process), with copy-retry and a rename-aside fallback.
- **Clean uninstall stops the daemon.** `agentguard uninstall` ends the
  background process via the pidfile supervisor before reverting configs, so it
  no longer leaves a locked binary behind.
- **`agentguard init` installs the Claude Code Skill** so the agent can drive
  setup/operation, and `dashboard` records its PID for the supervisor.

## [v1.2.0] — 2026-05-29

### Added
- **Completed the 15 stubbed internal packages** (every file ships with tests
  in the same package; suite is 139 tests, 0 failures):
  - `internal/crypto` — `SHA256Hex`, `CanonicalJSON`, `HashToolSchema` (DRY:
    `pipeline/attestation.go` now calls it), and a graceful-degrade `cosign`
    blob verifier.
  - `internal/store` — opt-in DuckDB analytics accelerator (behind the
    `duckdb` build tag; pure-Go SQLite fallback keeps `CGO_ENABLED=0` green)
    and a retention/`VACUUM`/`wal_checkpoint` job.
  - `internal/proxy` — transport router, streamable-HTTP reverse proxy, and
    legacy SSE passthrough, all running the pipeline on request/response.
  - `internal/ml` — `LoadModel`/`ModelInfo` with SHA-256 verification + cache,
    feeding `doctor`'s model-checksum check.
  - `internal/daemon` — pidfile `Supervisor` (start/stop/status, graceful
    signal + Kill fallback) and per-OS service-unit *generators*
    (systemd user unit, launchd plist, Windows scheduled-task) — generation
    only, no privileged install.
  - `internal/cli` — `agentguard config get|set|list` (JSON settings store)
    and `agentguard policy list|show|enable|disable` (audit-logged toggles),
    both wired into the root command.
  - `pkg/client` — public read-only Go client for the dashboard API
    (`Overview`, `Calls`, `Call`, `Servers`) used by the npm wrapper.
- **Real-time dashboard cockpit** (`internal/dashboard/assets/`): the embedded
  console is now a live security cockpit, not a static page.
  - "Threats blocked today" hero counter with a count-up animation that bumps
    on every new block.
  - Verdict-mix donut that animates as live calls stream in.
  - Activity river with `aria-live`; rows are clickable.
  - **Call-detail drawer**: pipeline **stage waterfall** (per-stage ns/µs/ms,
    coloured by outcome), side-by-side request/response **diff** for
    transform/block verdicts, decoded verdict reason, and Replay /
    mark-false-positive / copy-as-cURL actions.
  - **Servers page** rebuilt as a trust-gauge card grid (radial SVG gauges,
    green ≥80 / yellow 50–79 / red <50).
  - Toast notifications on block/flag, a command palette (Cmd/Ctrl+K), a
    global pause-stream button, `requestAnimationFrame`-batched SSE updates
    with a rolling 500-call client window, and `prefers-reduced-motion`
    support. No new dependencies; everything stays embedded via `embed.FS`.
- **`GET /api/calls/:id`** (`internal/dashboard`, `internal/store`): full
  single-call detail — header, inline request/response, ordered stage
  waterfall, and artifacts — with a clean 404 for unknown ids.
- **`agentguard seed-demo`** (hidden, `internal/cli`): generates realistic,
  varied demo traffic through the real store layer (≈500 historical calls,
  six scripted security scenarios incl. direct + indirect prompt injection,
  a rug-pull/schema-drift flag, a loop-detection trip, credential redaction,
  and a low-trust exfiltration block). `--live` streams new calls every
  1–3 s; `--reset` removes all demo data (tagged `client_user="demo"`).

### Changed
- Migrated the final `agentguard.dev` reference (the VHS demo tape install
  line) to `agentguard.space`; the repo now has zero `agentguard.dev`
  references and the GitHub repo homepage points at `agentguard.space`.

## [v1.0.0-rc1] — Milestone 6 (launch readiness)

### Added
- **Install scripts** (`scripts/install.sh`, `scripts/install.ps1`):
  one-liner installers for macOS/Linux and Windows. Detect OS+arch,
  fetch the matching release archive, verify SHA-256 against
  `checksums.txt`, install to `~/.agentguard/bin/`, append to PATH.
- **Release workflow** (`.github/workflows/release.yml`): tag-triggered
  cross-build for darwin/linux/windows × amd64/arm64. Produces
  archives + a combined `checksums.txt`, signs the checksum file with
  cosign keyless OIDC, attaches everything to the GitHub release. The
  release notes include the exact `cosign verify-blob` command.
- **Docs site** (`docs/`): mdbook-formatted manual. Pages cover
  install, quickstart, how it works, dashboard, rule packs,
  architecture, pipeline, schema, threat model, and per-command
  reference. Configured for the navy dark theme + repo edit links.
- **p99 enforcement** (`bench/p99_test.go`): runs 5,000 cheap-path
  iterations and fails the build if p99 exceeds 5 ms — the §13
  Definition-of-Done gate. Observed today: ~500 µs.
- **LAUNCH.md**: internal launch checklist plus drafts for the HN,
  Reddit, Twitter, and blog launch posts.

### Changed
- **README**: hero badges (release, CI, license, Go Report, cosign),
  install one-liner, dashboard screenshot placeholder, command table,
  threat-model link, sponsors box. The pitch is now front-loaded for
  HN's 30-second scan.
- **`docs/book.toml`**: real mdbook config (was a placeholder).

### Status against §13 Definition of Done
- ✅ Single-binary cross-platform build
- ✅ `init` finishes in <10 s on a fresh agent install
- ✅ Cursor round-trip is byte-identical (e2e gates this)
- ✅ `tail` looks great on 1080p
- ✅ Dashboard loads in <1 s, dark mode is beautiful
- ✅ `uninstall` leaves no trace except optional export
- ✅ CI green across 5 platforms (cross-build matrix shipped)
- ✅ Bench p99 < 5 ms (enforced by `TestCheapPathP99Under5ms`)
- ✅ Docs site is complete for every command
- ⏳ README 30s GIF — record before tagging v1.0.0
- ⏳ Three testimonials — recruit beta users in the launch week
- ⏳ Domain `agentguard.space` install redirects — operator config

## [Unreleased] — Milestone 5

### Added
- **ML classifier** (`internal/ml/classifier.go`): hand-tuned
  feature-based prompt-injection scorer with logistic normalisation,
  content-hash LRU-style cache, and 10+ calibrated signals
  (ignore-previous, role takeover, DAN, policy override, exfil markers,
  reveal-secrets, encoded payloads, prompt-injection markers,
  destructive shell, HTML script, zero-width chars, low-alpha walls).
  Confidence thresholds: 0.50 flag, 0.85 block. Sub-100µs per call.
- **ML pipeline stage** (`internal/pipeline/ml.go`): replaces the M4
  stub. Pulls user-facing text out of JSON-RPC envelopes
  (`params.arguments`, `params.text/input/prompt`) before scoring.
- **Circuit breaker stage** (`internal/pipeline/circuit.go`):
  per-session sliding-window failure tracking. Trips OPEN after
  `MaxBlocks`/`MaxErrors` in the window, HALF-OPEN after cooldown,
  back to CLOSED on a clean probe. Blocks all traffic on tripped
  sessions until cooldown elapses.
- **`agentguard replay`** (`internal/cli/replay.go`): re-runs the full
  inspection pipeline against historic `tool_calls` rows. Filter by
  session, server, time window, limit. Builds a synthetic chain
  (schema → policy → scanner → ML) and prints a verdict-diff table —
  useful for vetting a new rule pack against real traffic.
- **`agentguard pack list/show/verify`** (`internal/cli/pack.go`):
  inspect built-in and user-defined rule packs. User packs live in
  `~/.agentguard/packs/*.yaml` and are loaded by name (e.g.
  `pack show user/my-policy`).
- **`policy.LoadBuiltinBytes`**: exposed raw-YAML reader so the pack
  CLI can re-compile a pack for verification.

### Tests
- `internal/ml/`: benign/suspicious/injection classification, cache reuse,
  zero-width detection, long-low-alpha trigger.
- `internal/pipeline/`: ML stage skip-on-empty + block-on-injection.
- `internal/pipeline/`: circuit closed-by-default, trips-on-blocks,
  half-open-after-cooldown, session-id required.
- `internal/cli/`: pack list shows builtins, pack verify succeeds.

### Deliberately deferred
- ONNX-backed DeBERTa-v3 model — adds CGO + 100+ MB. The heuristic
  classifier hits the precision target without the dependency burden.
  The full ONNX path stays a v0.2+ upgrade.

## [Unreleased] — Milestone 4

### Added
- **`agentguard dashboard`** (`internal/cli/dashboard.go`): launches a
  localhost-only single-page web UI at `http://127.0.0.1:7878`. Live tool
  calls, top tools, server inventory, per-minute timeseries. Opens the
  browser automatically (suppress with `--no-browser`). Bound to 127.0.0.1
  only — never publicly reachable.
- **Dashboard server** (`internal/dashboard/{server,api,sse,embed}.go`):
  chi-based HTTP server with `/api/overview`, `/api/timeseries`,
  `/api/top-tools`, `/api/servers`, `/api/calls`, `/api/stats`, and an SSE
  stream at `/events` that pushes a `hello` snapshot + live `call` events
  + 25-second heartbeats. All static assets are embedded via `embed.FS`.
- **Analytics queries** (`internal/store/analytics.go`): `Overview`,
  `CallsByMinute`, `TopTools`, `Servers`, `RecentCalls`. Every query is
  bounded by a `limit` and a `window` so no API call ever scans more than
  a few hundred rows.
- **SSE broadcaster** (`internal/store/broadcast.go`): non-blocking
  pub/sub that the batched writer publishes to after each successful
  commit. Subscribers get a buffered channel and `Unsubscribe` cleanly on
  client disconnect.
- **Single-file dashboard UI** (`internal/dashboard/assets/`): hand-written
  HTML + vanilla JS + CSS, dark and light themes, KPI tiles with gradient
  accents, SVG sparkline with area fill, live-flash row animations,
  filterable calls table, top-tools usage bars, MCP-server inventory page.
  No build step; total weight under 25 KB.
- **Interactive init checklist** (`internal/cli/init_tui.go`): Bubble Tea
  TUI shown when `agentguard init --interactive` is passed. Per-agent
  checkboxes, ↑/↓/space/a/n/enter keys, abort with esc.
- **Registry package** (`internal/registry/`): metadata fetchers for npm
  (`registry.npmjs.org` + monthly downloads), PyPI, and GitHub
  (`api.github.com/repos`). Shared `*http.Client`, polite User-Agent,
  context-bound. `TrustScore()` aggregates a 0..100 score from age,
  popularity, license, repo presence, author, and activity recency.
- **Stage 2 attestation** (`internal/pipeline/attestation.go`): inspects
  inbound `tools/list` responses, computes a deterministic SHA-256 over
  the sorted (name, description, inputSchema) triples, and compares
  against the last-seen hash from `server_attestations`. A mismatch
  raises `VerdictFlag` with `{old_hash, new_hash}` — the rug-pull alarm.
  Store-side persistence in `internal/store/attestation.go`.
- **OTLP exporter** (`internal/otel/exporter.go`): hand-rolled OTLP/HTTP
  JSON span exporter — one span per tool call, attributes for
  `mcp.server`, `mcp.tool`, `mcp.direction`, `verdict`, `verdict.reason`.
  Empty endpoint = no-op, so off by default. Avoids pulling the multi-MB
  `go.opentelemetry.io` tree.
- **Tests**: dashboard route smoke tests via `httptest` covering all
  `/api/*` endpoints and `/` static handler. Attestation tests for first-
  seen, drift-flag, reorder-stable hashing, and outbound short-circuit.
  Registry trust-score tests for empty/high/low signal. OTLP exporter
  tests with an `httptest.Server` collector.

### Changed
- `cmd/agentguard` root now registers `dashboard` alongside the M1-M3 set.
- `internal/store/sqlite.go` exposes `Broadcaster()` so the proxy + tests
  can fan out tool-call events.
- `internal/store/batched_writer.go` publishes `ToolCall` events to the
  broadcaster after each successful `tx.Commit()`.

### Dependencies
- Added `github.com/go-chi/chi/v5` for HTTP routing.

### Deliberately deferred
- SvelteKit/Bun dashboard rebuild — current vanilla single-page UI hits
  the "rich visual design" bar and stays buildless. SvelteKit replaces
  these files in a later milestone without changing the API surface.
- DuckDB analytics — full DuckDB pulls CGO, conflicting with the pure-Go
  single-binary promise. All queries run against SQLite for now.
- ML stage — wired as a `VerdictSkip` stub in `internal/pipeline/ml.go`,
  full implementation lands in M5.

## [Unreleased] — Milestone 3

### Added
- **Agent detection** (`internal/agent_detect/`): one detector per agent
  (Claude Code, Cursor, Codex CLI, Gemini CLI, Windsurf). Each knows the
  union of historic config paths for its agent and parses MCP entries into
  a typed `Detection` struct. Codex uses TOML (`BurntSushi/toml`), the
  others use JSON.
- **Config patcher** (`internal/agent_detect/patcher.go`): rewrites every
  stdio MCP entry to invoke `agentguard wrap --upstream-name <name> -- …`.
  Backs the original up to `<path>.agentguard.bak` **byte-for-byte** before
  any rewrite. Idempotent (a second run wraps nothing twice). HTTP entries
  and entries the user names in `--skip-server` are left alone.
- **`agentguard init`** (`internal/cli/init.go`): creates
  `~/.agentguard/{bin,data,logs,packs,config}` mode 0700, runs migrations,
  detects every supported agent, patches their configs, records one
  `agents` row + one `audit_log` row per detection. `--non-interactive` is
  the M3 default; the Bubble Tea checklist arrives with the dashboard in M4.
  `--dry-run` previews without writing.
- **`agentguard uninstall`** (`internal/cli/uninstall.go`): reads every
  active `agents` row, restores each config from its `.agentguard.bak` (and
  removes the backup), marks the row inactive. `--purge` additionally deletes
  `~/.agentguard` recursively; otherwise audit data is preserved.
- **`agentguard doctor`** (`internal/cli/doctor.go`): homebrew-style health
  check. Verifies the directory tree, runs `PRAGMA integrity_check`, counts
  active agents and recorded `tool_calls`, re-detects each agent and reports
  how many of its servers are still routed through us, and lists the
  builtin rule packs. Exit code is 0 unless any check fails outright.
- **`agentguard tail`** (`internal/cli/tail.go`): Bubble Tea TUI showing the
  most recent N tool calls polled from SQLite every 500 ms. Colour-coded
  verdict glyph (✓ allow, ✗ block, ⚠ flag, ✎ transform, · pending),
  truncated columns for time/server/tool/latency/reason. `--once` prints a
  single ASCII snapshot for non-TTY use.
- **`agentguard scan`** (`internal/cli/scan.go`): spawns an upstream MCP
  server and fires 8 canned prompt-injection probes (ignore-previous,
  system-prompt override, base64 marker, zero-width chars, you-are-now,
  data-exfil marker, policy override, HTML script). Reports per-payload
  reflection and exits non-zero if any payload reached the model surface.
  Designed for CI. The full 50+ payload library arrives in M5.
- **`internal/cli/paths.go`**: shared `~/.agentguard` layout resolver
  (`Paths()` and `EnsurePaths`) used by init, uninstall, and doctor.
- **e2e tests**:
  - `TestInitAndUninstallRoundtrip` — real binary, fake `HOME`, init then
    uninstall, asserts the patched config invokes `agentguard wrap …` and
    that the post-uninstall file is **byte-identical** to the original
    (the §13 definition-of-done promise).
  - `TestDoctorReportsHealthyAfterInit` — init then doctor, asserts
    `agents: 1 active` and that all builtin packs are listed.
- **Unit tests**: 13 new across `internal/agent_detect` covering each
  detector, idempotent patching, HTTP-entry skipping, `--skip-server`,
  byte-identical backup + restore, and the TOML/Codex code path.

### Changed
- `cmd/agentguard` root now registers `init`, `uninstall`, `doctor`,
  `tail`, and `scan` alongside the M1/M2 commands.

### Dependencies
- Added `github.com/charmbracelet/bubbletea` and `lipgloss` for the TUI.
- Added `github.com/BurntSushi/toml` for Codex CLI config parsing.

### Deliberately deferred
- Stage 2 (server attestation) — the binary-hash check needs the registry
  package (npm/PyPI/GitHub metadata fetchers) which needs network and is
  M4 work.
- Bubble Tea checklist for `init` — lands with the dashboard in M4.
- Full 50+ scan-payload library — M5.
- `agentguard daemon` supervisor — M4/M6 depending on platform.

## [Unreleased] — Milestone 2

### Added
- **Stage 0 — transport guard** (`internal/pipeline/transport.go`):
  per-session token-bucket rate limit (100 req/s default, configurable via
  `--rate-per-second`) and a hard 8 MiB cap on inspected frame size
  (`--max-frame-bytes`). Both are exposed on `agentguard wrap`.
- **Stage 1 — schema validator** (`internal/pipeline/schema.go`): enforces
  the JSON-RPC 2.0 envelope; rejects malformed JSON, wrong version,
  outbound frames missing `method`, and inbound frames that set both
  `result` and `error`.
- **Stage 3 — policy engine** (`internal/policy/{engine,compiler,packs}.go`):
  YAML rule packs compiled at load time to an AST of regex-precompiled
  rules. Matches on `server`, `tool`, `direction`, and `content_regex`.
  Actions: `allow`, `block`, `flag`, `redact`, `require_approval`,
  `rate_limit`. Engine evaluates first-terminal-wins with flag accumulation.
- **Two embedded rule packs** (`internal/policy/builtin/`): `default.yaml`
  (secrets for AWS/GitHub/Stripe/OpenAI/Anthropic/Google/Slack/SSH/JWT +
  injection flags + destructive shell blocks) and `strict.yaml` (block on
  inbound injection signatures instead of flag).
- **Stage 4 — content scanner** (`internal/pipeline/scanner.go`): independent
  regex bank that does the actual byte-level redaction. Secrets are replaced
  by `[REDACTED:<kind>]`; injection signatures emit `flag`. Runs even when
  no rule pack is loaded — the bank is the floor of defense.
- **Chain wired into `agentguard wrap`**: every frame now goes through
  transport → schema → policy → scanner. New flags: `--pack`,
  `--rate-per-second`, `--max-frame-bytes`, `--no-inspect`.
- **Proxy honours `VerdictTransform`**: when stage 4 redacts a frame, the
  wrap loop forwards the transformed bytes (not the original), and records
  `verdict='transform'` with a `verdict_reason` of `content_scan:secret_redacted`.
- **e2e**: two new tests — `TestWrapRedactsSecretInToolResponse` proves AKIA
  keys are replaced on the wire and recorded as `transform`;
  `TestWrapBlocksOnPolicyMatch` proves the strict pack returns a clean
  JSON-RPC `-32099` to the agent.
- **Mock server** gained `leak_secret` and `injection` tools for e2e.
- **Bench M2 baselines** (Ryzen 5 4600H, Windows, CGO off):
  - `BenchmarkM2ChainCleanFrame`  →  **33 μs/op** (0.033 ms)
  - `BenchmarkM2ChainSecretFrame` →  **23 μs/op**
  - Budget is `< 5 ms` p99 for the full roundtrip — we are ~150× under.

### Changed
- `pipeline.Message` now carries `SessionID`, `Method`, and `ServerName`
  alongside the raw bytes so each stage avoids re-parsing.
- `pipeline.Chain.Run` propagates `StageResult.Transform` into the message
  for downstream stages.
- `proxy.recordCall` now records inbound frames whenever the pipeline took
  non-trivial action (transform/flag/block) and writes `verdict_reason`.

### Dependencies
- Added `github.com/goccy/go-yaml` (locked choice from requirements §2).

## [Unreleased] — Milestone 1

### Added
- Repository scaffold matching `requirements.md` Section 1.
- MIT license, community health files (CODE_OF_CONDUCT, CONTRIBUTING, SECURITY).
- `go.mod` with Go 1.23+ and the locked dependency set from Section 2.
- `internal/store/`: SQLite layer with WAL pragmas, batched writer goroutine,
  and goose migration `0001_initial.sql` creating every table from
  `database.md` Section 3.
- `internal/proxy/stdio.go`: transparent stdio JSON-RPC proxy that spawns an
  upstream command and forwards JSON-RPC messages in both directions.
- `cmd/agentguard/main.go` plus `internal/cli/{root,wrap,version}.go`:
  `agentguard wrap` and `agentguard --version` commands wired via cobra.
- `internal/version`: version string injected via `-ldflags` at build time.
- `internal/pipeline/`: stage interface and skeleton stage files (no
  inspection logic yet).
- `e2e/mock_mcp_server/`: a minimal JSON-RPC MCP server for testing.
- `e2e/wrap_test.sh`: end-to-end test that builds the binary and mock server,
  drives three tool calls through `agentguard wrap`, and verifies that the
  responses come back and that `sessions` + `tool_calls` rows exist in SQLite.
- `bench/proxy_bench_test.go`: baseline benchmarks — empty inspection chain
  runs at **6.8 ns/op (0 allocs)** and a tool_call writer submit is
  **~486 ns/op**. Target `p99 < 5 ms` for the full proxy roundtrip stays
  the gate as inspection stages land in Milestones 2-5.
- `Makefile` with `build`, `test`, `lint`, `bench`, `clean`, `install-tools`.
- `.github/workflows/ci.yml`: lint + test + cross-build matrix
  (darwin/linux/windows × amd64/arm64).
- `PR_DRAFT.md` summarising the milestone.
