# Changelog

All notable changes to AgentGuard are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)
and this project adheres to [Semantic Versioning](https://semver.org/).

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
