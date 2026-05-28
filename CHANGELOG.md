# Changelog

All notable changes to AgentGuard are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)
and this project adheres to [Semantic Versioning](https://semver.org/).

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
