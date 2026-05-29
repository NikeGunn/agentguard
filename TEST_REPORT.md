# AgentGuard — Verification Report

> Phase 6 of `COMPLETION_PLAN.md`. Every check below was actually run on this
> machine; numbers are copied from real output, and environment-limited checks
> are marked honestly rather than claimed.

**Environment:** Windows 10 (10.0.19045), `go1.25.7 windows/amd64`,
AMD Ryzen 5 4600H (12 logical CPUs), `CGO_ENABLED=0`, no local C toolchain.

## 1. Build & vet

| Check | Command | Result |
|-------|---------|--------|
| Build | `go build ./...` | ✅ pass (exit 0) |
| Vet | `go vet ./...` | ✅ pass (no diagnostics) |
| Tidy | `go mod tidy` then no-diff re-run | ✅ idempotent / stable |

`go mod tidy` pulled the `marcboeker/go-duckdb` dependency tree, which is
correct: it is referenced only by `internal/store/duckdb_cgo.go` (behind the
`duckdb` build tag). The default, always-compiled path
(`duckdb_default.go`) is pure-Go and CGO-free, so `CGO_ENABLED=0`
cross-compilation stays green.

## 2. Test suite

`go test ./... -count=1` → **all packages ok, 0 failures.**

- **139 top-level tests** (159 including subtests), every one `--- PASS`.
- Includes the 7 newly completed stub packages' tests: `internal/ml`,
  `internal/daemon` (supervisor + units), `internal/cli` (config + policy),
  `pkg/client`, plus the previously-landed `crypto`, `store`, `proxy`.

| Package | Status |
|---------|--------|
| `bench`, `e2e`, `agent_detect`, `cli`, `crypto`, `daemon`, `dashboard`, `ml`, `otel`, `pipeline`, `policy`, `proxy`, `registry`, `store`, `pkg/client` | ✅ ok |

### Coverage (key packages, per plan §57)

| Package | Coverage |
|---------|----------|
| `internal/pipeline` | **78.4%** of statements |
| `internal/policy`   | **78.1%** of statements |

## 3. Fuzz smoke — JSON-RPC parser

`go test ./internal/proxy -run=^$ -fuzz=FuzzParseRPC -fuzztime=10s`

- Target: `parseRPC` + `rpcFrame.toolName` (the untrusted HTTP-transport
  envelope decoder), in `internal/proxy/fuzz_test.go`.
- **~214,000 executions, 168 interesting inputs, 0 crashes.** ✅

## 4. Benchmarks (cheap-path p99 target < 5ms, plan §59)

`go test ./bench -bench=. -benchmem -run=^$`

| Benchmark | ns/op | B/op | allocs/op |
|-----------|------:|-----:|----------:|
| `PipelineChainEmpty` (cheap path) | **6.54 ns** | 0 | 0 |
| `M2ChainCleanFrame` | 33,546 ns (~0.034 ms) | 2,331 | 21 |
| `M2ChainSecretFrame` | 23,801 ns (~0.024 ms) | 3,889 | 45 |
| `StoreSubmit` | 624.5 ns | 224 | 2 |

The cheap path is **~6.5 ns**, and even a full secret-scanning frame is
**~0.034 ms** — roughly **150× under** the 5 ms budget. The dedicated
`TestCheapPathP99Under5ms` test also passes.

## 5. Vulnerability scan

`govulncheck ./...` → **8 findings, ALL in the Go standard library**, all
fixed in `go1.25.8` (e.g. GO-2026-4601 net/url IPv6, GO-2026-4602 os Root).

- **Zero** vulnerabilities in AgentGuard's own code or in any directly-called
  third-party package.
- These are toolchain-patch issues, not code issues. CI pins
  `go-version: "1.25.x"` with `check-latest: true`, so GitHub's runner builds
  with ≥ go1.25.8 and the findings clear automatically. Locally the toolchain
  is 1.25.7 (1.25.8 not installed here).

## 6. Lint gate (golangci-lint v2.5.0 — CI's configured linters)

`golangci-lint` is not installed on this machine, so the individual linters it
runs were exercised directly:

| Linter | Command | Result on new code |
|--------|---------|--------------------|
| gofmt | `gofmt -l <new files>` | ✅ clean |
| staticcheck | `staticcheck ./internal/{cli,daemon,proxy} ./pkg/client` | ✅ only the pre-existing `scan.go` `ST1018`, which `.golangci.yml` explicitly excludes |
| errcheck | `errcheck ./...` | ✅ all hits are `fmt.Fprintf*` / `*.Close()` — every one on the `.golangci.yml` `exclude-functions` allowlist; **none in new files** |

## 7. CI parity (`.github/workflows/ci.yml`)

The workflow gates on: `go mod tidy` diff, `go vet`, `golangci-lint`,
`go test -race -coverprofile`, `bash ./e2e/wrap_test.sh`, and a 6-target
cross-build. Locally:

- tidy / vet / lint-equivalents / full test suite / e2e-as-Go-test: ✅ pass.
- `go test -race` and the `duckdb`-tagged build require a C toolchain
  (`gcc`), which is **absent on this Windows box** — both error with the
  expected "race/cgo requires a C compiler" message, not a code failure.
  GitHub's `ubuntu-latest` runner ships gcc, so both run there.

## Honest known-limitations

1. `-race` and CGO/`duckdb`-tagged builds unverified locally (no gcc); deferred
   to CI, which has the toolchain.
2. `govulncheck` stdlib findings clear only once the runner uses go1.25.8+
   (CI's `1.25.x` + `check-latest` does this); not reproducible on the 1.25.7
   toolchain installed here.
3. golangci-lint run by-proxy (gofmt + staticcheck + errcheck) rather than the
   single binary, which is not installed locally.
