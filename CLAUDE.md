# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

AgentGuard is a single Go binary that acts as a zero-trust security sidecar for AI agents (Claude Code, Cursor, Codex, Gemini CLI, Windsurf — anything speaking MCP). It inserts itself transparently between an agent and the MCP servers it calls by rewriting each agent's config so the agent spawns `agentguard wrap -- <original command>` instead of the upstream server directly. Every JSON-RPC frame then flows through an inspection pipeline before reaching either side.

Module path: `github.com/agentguard/agentguard`. Go 1.25.x. Local-only, no daemon, no cloud, no telemetry.

## Build / test / lint

```sh
make build         # -> ./bin/agentguard (stamps version via ldflags)
make test          # unit + integration with -race (needs CGO; Linux/macOS)
make test-norace   # Windows-friendly: no -race, no CGO needed
make lint          # go vet + golangci-lint v2 (falls back to vet if not installed)
make bench         # benchmark suite + p99 gate under ./bench
make e2e           # shell-driven e2e (POSIX only): builds binary + mock, runs e2e/wrap_test.sh
make install-tools # one-time: golangci-lint v2 + goose
```

Run a single test: `go test -run TestName ./internal/<pkg>/`. CI's lint is `golangci-lint run --timeout=5m` against `.golangci.yml` (v2 config — the action is pinned to `golangci-lint-action@v8` with `version: v2.5.0`; a v1 mismatch produces `exit 3`).

**Windows note:** the race detector needs CGO and a C compiler, which dev Windows boxes typically lack — use `make test-norace` (or `go test ./...`) locally; Linux CI is the source of truth for race results. `modernc.org/sqlite` is pure-Go, so default builds need no CGO.

## Architecture

The data path is: `agent → agentguard wrap → MCP server` and back, with inspection in the middle.

- **`cmd/agentguard`** — entry point; delegates to `internal/cli`.
- **`internal/cli`** — Cobra commands: `init`, `dashboard`, `tail`, `scan`, `doctor`, `replay`, `pack`, `wrap`, `uninstall`, etc. `root.go` wires them together. `wrap.go`'s `buildChain()` is where the pipeline stage order is defined. `paths.go` owns the canonical `~/.agentguard/` layout (`bin/ data/ logs/ packs/ config/`, all mode 0700).
- **`internal/proxy`** — the stdio JSON-RPC pump (`stdio.go`) plus SSE and streamable-HTTP transports. Parses each frame **once** into a `pipeline.Message` and hands it to the chain.
- **`internal/pipeline`** — the inspection engine. `stage.go` defines the `Stage` interface (`Name()` + `Run(ctx, *Message) StageResult`) and the `Chain` runner. Stages: `TransportGuard` → `SchemaValidator` → `PolicyStage` → `ContentScanner` (the "cheap path" wired in `buildChain`), plus `attestation`, `circuit` (per-session circuit breaker), and `ml` (heuristic prompt-injection classifier in `internal/ml`). `Chain.Run` short-circuits on the first `Block`/`Error`, `Transform` mutates `Message.Raw` and continues, `Flag` is informational; the final verdict is the most severe seen.
- **`internal/policy`** — YAML rule-pack engine + builtin packs (`internal/policy/builtin/*.yaml`). User packs in `~/.agentguard/packs/*.yaml` load at runtime, no recompile.
- **`internal/store`** — SQLite layer (`modernc.org/sqlite`, pure-Go). Migrations are goose SQL files embedded via `//go:embed migrations/*.sql`. A batched-writer goroutine funnels proxy events into the DB. Connection pragmas (WAL, etc.) live in `sqlite.go`. There is an optional DuckDB read-only analytics path gated behind `-tags duckdb` (`duckdb_cgo.go` vs `duckdb_default.go`) — do not assume it compiles by default.
- **`internal/dashboard`** — chi HTTP server + SSE live updates; the SPA is embedded via `//go:embed assets/*`. Serves `127.0.0.1:7878`.
- **`internal/agent_detect`** — per-agent config detectors + the patcher that rewrites MCP entries. `detect.go`'s `DetectAll()` is the registry; each agent (cursor, codex, …) is one file.
- **`internal/registry`** — npm/PyPI/GitHub metadata fetch + trust scoring.
- **`internal/otel`** — OTLP/HTTP span exporter. **`internal/version`** — the ldflags stamp target.

`e2e/` holds the shell e2e test and `mock_mcp_server`. `bench/` holds the p99 performance gate. `docs/` is mdbook source; `site/` is the landing page (agentguard.space); `demo/` is the vhs tape + live-demo driver.

## Key invariants

- **The proxy must stay transparent.** Neither agent nor server should be able to tell `wrap` is in the path. Backups taken by `init` are byte-identical and `uninstall` restores them exactly.
- **Stages parse nothing twice.** The proxy fills `Message` fields once; stages read `Message`, they don't re-parse `Raw` from scratch unless necessary. `Raw` always holds exact wire bytes (newline included); a `Transform` verdict replaces `Raw`.
- **There is a measured <5 ms p99 budget on the cheap path, enforced by a CI bench gate.** Don't add unbounded work to `TransportGuard`/`SchemaValidator`/`PolicyStage`/`ContentScanner`.
- **Source on `main` ≠ the released binary.** Users get the binary from the latest git **tag** via the install script hitting the GitHub "latest release" API. A user-facing fix isn't shipped until a new `v*` tag is pushed (release workflow cross-builds 6 targets and cosign-signs `checksums.txt`).

## Conventions

Conventional Commits with a package scope: `feat(proxy):`, `fix(cli):`, `chore(store):`, `docs(...)`, `ci(...)`, `deps(...)`, `test(...)`. Scope is the affected area (`proxy`, `cli`, `dashboard`, `store`, `pipeline`, `site`, `release`, …). `main` has branch protection: linear history (no merge commits), no force-push, required status checks (`Lint and test`, `e2e (POSIX shell)`, 6× `Cross-build`), conversation resolution required.

`DEVELOPER_MANUAL.md` is the operator playbook (releasing, CI debugging, adding detectors/packs, demo) with copy-paste PowerShell. `database.md` documents the store schema and pragmas. `CHANGELOG.md` has the milestone-by-milestone rationale.

## Common extension points

- **New agent detector:** copy `internal/agent_detect/<existing>.go`, edit Kind/paths/server-array key, register in `detect.go`'s `DetectAll()`, add a `_test.go` with fixtures.
- **New rule pack:** drop YAML in `internal/policy/builtin/<name>.yaml` (builtin, needs recompile) or `~/.agentguard/packs/` (user, runtime). Verify with `agentguard pack verify <name>`.
