# AgentGuard — Completion Plan

> Derived from `AUDIT.md`. Ordered, dependency-aware, ticked as work lands.
> Each stub file ships with a test in the same package (requirements.md §7).
> Conventional Commits, one logical change per commit, on
> `feat/complete-stubs-and-verify` (never `main`).

## Phase 0 — Audit & plan
- [x] Vendor `requirements.md`, `system-design.md`, `database.md`, `prompt-for-claude-code.md` into repo root.
- [x] Write `AUDIT.md` (file table, Section-3 map, domain proof).
- [x] Write this plan.
- [x] Remove build junk (`agentguard.exe~*`, temp dir).

## Phase 1 — Domain migration
- [x] **Already complete** (verified in AUDIT.md §3). Zero `.dev` refs in shipping surfaces.

## Phase 2 — Fill the 15 stub files (DRY, tested)

Ordered so shared helpers land before their consumers.

### 2a. crypto (foundation — reused by attestation & registry)
- [ ] `crypto/hashing.go`: `SHA256Hex`, `CanonicalJSON`, `HashToolSchema` (the canonical (name,desc,inputSchema) hash currently inline in `pipeline/attestation.go`).
- [ ] `crypto/cosign.go`: `Verifier` wrapping the `cosign` CLI; `Available()`, `VerifyBlob`; graceful degrade (returns `ErrCosignUnavailable`) when binary absent.
- [ ] `crypto/hashing_test.go`, `crypto/cosign_test.go`.
- [ ] Refactor `pipeline/attestation.go` to call `crypto.HashToolSchema` (DRY; behaviour-preserving).

### 2b. store (analytics + lifecycle)
- [ ] `store/duckdb.go`: `Analytics` attach SQLite read-only via go-duckdb; `Available()` probe; fall back to the existing SQLite analytics when the driver/CGO isn't present (pure-Go build stays green).
- [ ] `store/retention.go`: `RetentionJob` — delete rows older than N days, `wal_checkpoint(TRUNCATE)`, weekly `VACUUM`; `RunOnce` + scheduled `Start(ctx)`.
- [ ] `store/duckdb_test.go`, `store/retention_test.go`.

### 2c. proxy (transports + router)
- [ ] `proxy/router.go`: `Transport` enum + `SelectChain` / `Route` choosing the inspection chain by transport & direction.
- [ ] `proxy/streamable_http.go`: `HTTPProxy` — reverse proxy that runs the pipeline on request & response bodies, bound to 127.0.0.1.
- [ ] `proxy/sse.go`: legacy SSE passthrough that frames `data:` events through the pipeline.
- [ ] `proxy/router_test.go`, `proxy/streamable_http_test.go`, `proxy/sse_test.go`.

### 2d. ml model loader
- [x] `ml/models.go`: `LoadModel` / `ModelInfo` (path + SHA-256 + presence), cache, `DefaultModelPath`. Feeds `doctor`'s model-checksum check.
- [x] `ml/models_test.go`.

### 2e. daemon
- [x] `daemon/supervisor.go`: `Supervisor` — pidfile-based start/stop/status, cross-platform process spawn, graceful signal handling.
- [x] `daemon/launchd.go` / `systemd.go` / `windows_svc.go`: per-OS service-unit *generation* (`Unit()` returns the file body + install path). No privileged install (respects §12).
- [x] `daemon/supervisor_test.go`, `daemon/units_test.go`.

### 2f. cli config + policy (wire into root)
- [x] `cli/config.go`: `config get|set|list` over a small JSON config store (retention.days, telemetry, theme, cloud-sync). Wire `newConfigCmd` into `root.go`.
- [x] `cli/policy.go`: `policy list|show|enable|disable` over the `policies`/`policy_rules` tables, writing `audit_log`. Wire `newPolicyCmd` into `root.go`. (`set` is covered by the YAML `pack` workflow; this command owns activation, matching the dashboard toggles.)
- [x] `cli/config_test.go`, `cli/policy_test.go`.

### 2g. public client
- [x] `pkg/client/client.go`: `Client` over the local dashboard REST API (`Overview`, `Calls`, `Call`, `Servers`); used by the npm wrapper. `client_test.go` against `httptest`.

## Phase 6 — Verification battery (run, don't assume)
- [x] `go build ./...`, `go vet ./...`. — both pass (exit 0).
- [x] `go test ./... -count=1` — 139 tests pass (159 incl. subtests), 0 fail; coverage pipeline 78.4% / policy 78.1%.
- [x] `go test -fuzz` smoke on JSON-RPC parser (short). — ~214k execs, 0 crashes.
- [x] `go test -bench` proxy/pipeline — cheap-path 6.5ns, secret-frame ~0.034ms (≫150× under 5ms). Numbers in TEST_REPORT.md.
- [x] `go install golang.org/x/vuln/cmd/govulncheck@latest && govulncheck ./...`. — 8 findings, all stdlib, fixed in go1.25.8 (CI's `1.25.x` clears them); zero in our code.
- [x] golangci-lint linters run by-proxy (gofmt + staticcheck + errcheck) — clean on new code; binary not installed locally. (gosec/gitleaks/Playwright deferred — env-limited, noted honestly in TEST_REPORT.)
- [x] Write `TEST_REPORT.md` (every check, real numbers, honest known-failures).

## Phase 7 — FAANG-level demo GIFs + README
- [ ] `demo/tapes/{install,tail,scan,doctor}.tape` — polished VHS scripts, dark theme, ≥14pt, 2x.
- [ ] `demo/playwright/record-dashboard.mjs` — records Overview→/calls/:id WebM, ffmpeg+gifski → GIF.
- [ ] `scripts/render-demos.sh` — one command to render all GIFs (VHS + Playwright).
- [ ] Render whatever this environment allows; clearly mark rendered vs scripted in TEST_REPORT.md.
- [ ] Update `README.md`: hero GIF + per-feature GIFs, image manifest printed.
- [ ] Update `CHANGELOG.md` `## [Unreleased]`.

## Phase 8 — Deliverables
- [ ] `AUDIT.md`, `COMPLETION_PLAN.md` (ticked), `TEST_REPORT.md`, `OPEN_QUESTIONS.md` current.
- [ ] Final summary with real numbers + honest status.
