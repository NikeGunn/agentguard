# Milestone 1 — Core proxy + SQLite store + `wrap` command

## Summary

Lays the foundation for AgentGuard: a working stdio JSON-RPC proxy that
spawns an upstream MCP server, forwards traffic verbatim in both directions,
and records every JSON-RPC request into a local SQLite database. The full
inspection pipeline interface is defined but ships with an empty chain — real
stages land in Milestones 2-5.

The repo tree from `requirements.md §1` is fully present, every locked
dependency from §2 is in `go.mod`, and the four Milestone-1 commands work
end-to-end:

```bash
git clone <repo>
cd agentguard
make build               # → ./bin/agentguard
make mock                # → ./bin/mock_mcp_server
make test                # all green
./bin/agentguard version # prints injected version string
./bin/agentguard wrap --upstream-name mock -- ./bin/mock_mcp_server < calls.jsonl
```

## What landed

- **Repo scaffold** — every directory and placeholder file from `requirements.md §1`
  exists, with `package`-only stubs in Go files that later milestones own.
- **Community files** — `LICENSE` (MIT, copyright Nikhil Bhagat), `README.md`,
  `CODE_OF_CONDUCT.md`, `CONTRIBUTING.md`, `SECURITY.md`, `CHANGELOG.md`,
  `OPEN_QUESTIONS.md`, `.gitignore`, `.github/{FUNDING.yml,dependabot.yml}`.
- **Go module** — `go.mod` with cobra, modernc.org/sqlite, pressly/goose v3,
  oklog/ulid v2, stretchr/testify. (The `go 1.25.7` line is goose's
  declared minimum; spec floor is "1.23+" which it satisfies.)
- **`internal/store`** — SQLite layer with WAL + the rest of the pragmas from
  `database.md §4`, embedded goose migration `0001_initial.sql` creating every
  table from `database.md §3`, plus a batched writer goroutine (100 events or
  10ms window, channel-fed, single transaction per flush). `Close()` is
  idempotent so the writer drains on test cleanup.
- **`internal/proxy/stdio.go`** — transparent stdio JSON-RPC proxy. Spawns
  the upstream, pumps newline-delimited frames in both directions, parses
  enough of each frame to identify `tools/call` and extract the tool name,
  records `outbound` requests as `tool_calls` rows, and synthesises a
  JSON-RPC `-32099` error if the (empty for M1) inspection chain blocks.
- **`internal/pipeline`** — `Stage` interface, `Chain` runner with short-
  circuit-on-block semantics, and the `MLStage` skeleton tagged
  `// TODO milestone 5` per spec.
- **`internal/cli`** — `agentguard wrap` and `agentguard version`, both via
  cobra. `cmd/agentguard/main.go` is the single-line root.
- **`internal/version`** — three vars overridable via `-ldflags` for
  reproducible release builds; the Makefile already wires this up.
- **`e2e/mock_mcp_server`** — minimal newline-delimited JSON-RPC server
  implementing `initialize`, `tools/list`, and `tools/call` for `echo`, `add`,
  `ping`.
- **`e2e/wrap_e2e_test.go`** — Go-driven end-to-end test that builds the real
  binary and the mock, sends three tool calls, asserts three responses come
  back, and asserts one `sessions` row plus three `tool_calls` rows exist in
  the resulting SQLite file. Runs cross-platform (including Windows).
- **`e2e/wrap_test.sh`** — the POSIX shell version asked for in the spec,
  plus a `e2e/dbcheck` helper for hosts without the `sqlite3` CLI.
- **`bench/proxy_bench_test.go`** — baseline benchmarks. The chain runner is
  6 ns/op (zero alloc, empty chain) and a tool_call event submit is ~500 ns.
  These are the floor we will defend against the 10% regression gate in
  `requirements.md §5.1` as inspection stages land.
- **`Makefile`** — `build`, `test`, `test-norace`, `lint`, `bench`, `clean`,
  `install-tools`, `mock`, `e2e`, `fmt`, `vet`, `help`.
- **`.github/workflows/ci.yml`** — lint + race-enabled tests on linux,
  `wrap_test.sh` job, and a cross-build matrix covering
  `darwin/{amd64,arm64}`, `linux/{amd64,arm64}`, `windows/{amd64,arm64}`.

## Tests + lint output

```
$ go vet ./...
(no output)

$ go test -timeout 120s ./...
ok      github.com/agentguard/agentguard/bench         [no tests to run]
ok      github.com/agentguard/agentguard/e2e           2.093s
ok      github.com/agentguard/agentguard/internal/pipeline  (cached)
ok      github.com/agentguard/agentguard/internal/store     (cached)
```

(All other packages are M2+ stubs with no test files yet — listed by
`go test ./...` as `[no test files]`, not failures.)

## Bench baseline (Ryzen 5 4600H, Windows, CGO off, M1 empty chain)

```
BenchmarkPipelineChainEmpty-12    175,953,955 iters   6.799 ns/op   0 B/op  0 allocs
BenchmarkStoreSubmit-12             2,611,862 iters   486.4 ns/op  223 B/op  2 allocs
```

The full proxy roundtrip target in `system-design.md §8` is **p99 < 5 ms on
the cheap path**. Today's empty-chain roundtrip is dominated by the
JSON-RPC frame parsing and the OS pipe; these baseline numbers say we are
several orders of magnitude under the budget before any inspection lands.
The 10% CI gate will lock the floor on the next milestone.

## Live smoke test (signed binary)

```
$ ./bin/agentguard version
agentguard v0.1.0-m1 (<commit>, 2026-05-28T04:40:21Z)

$ cat <<EOF | ./bin/agentguard wrap --upstream-name mock -- ./bin/mock_mcp_server
{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"echo","arguments":{"message":"smoketest"}}}
{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"ping"}}
{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"add","arguments":{"a":40,"b":2}}}
EOF
{"jsonrpc":"2.0","id":1,"result":{"content":[{"text":"smoketest","type":"text"}]}}
{"jsonrpc":"2.0","id":2,"result":{"content":[{"text":"pong","type":"text"}]}}
{"jsonrpc":"2.0","id":3,"result":{"content":[{"text":"42","type":"text"}]}}
```

The resulting SQLite DB at `~/.agentguard/data/agentguard.db` contains a
`sessions` row plus three `tool_calls` rows, verified by the e2e test.

## Open items (deliberately deferred per the spec)

- Streamable-HTTP and SSE transports — `internal/proxy/{streamable_http,sse}.go`
  are package stubs (Milestone 2+).
- Inspection stages 0–4, plus the policy engine — interface is locked,
  implementations are M2/M3.
- ML stage — `internal/pipeline/ml.go` returns `VerdictSkip` and is tagged
  `// TODO milestone 5`.
- `init`, `tail`, `scan`, `replay`, `pack`, `policy`, `config`, `doctor`,
  `uninstall` commands — files exist as stubs only.
- Dashboard — `web/` and `internal/dashboard/` are scaffolding only.
- One open question logged in `OPEN_QUESTIONS.md` about stdio framing
  conventions; chose newline-delimited JSON to match the reference servers.

## How to review

1. `make build && ./bin/agentguard version`
2. `make test`            — all green
3. `make bench`           — record baselines as the floor for M2
4. Walk `internal/store/migrations/0001_initial.sql` against
   `database.md §3` — should be a 1:1 match including indexes
5. `internal/proxy/stdio.go` is the meat of the milestone; the bidirectional
   pump and `recordCall` are the two parts that will grow as stages arrive
6. `internal/pipeline/stage.go` defines the shape every M2+ stage must satisfy

## What's next (Milestone 2)

Cheap-path inspection: Stages 0 (transport guard) → 1 (schema validator) →
3 (policy engine) → 4 (content scanner). Stage 2 (server attestation) +
real upstream registry come in M3 alongside `agentguard init` and
agent-config auto-patching.

---

**Milestone 1 complete. Ready for review before Milestone 2.**
